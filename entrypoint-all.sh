#!/bin/sh
# All-in-one entrypoint: starts Asterisk + xpbx web server
set -e

# Source the Asterisk entrypoint to generate configs, but don't exec Asterisk yet.
# We override the exec at the end by running the script in a subshell up to the exec line.
# Simpler: just run the config generation parts inline.

# --- Asterisk config generation (from entrypoint.sh) ---

NAT_IP=$(echo "$EXTERNAL_IP" | cut -d',' -f1)

if [ -z "$NAT_IP" ]; then
  for STUN_SERVER in stun.l.google.com:19302 stun.cloudflare.com:3478; do
    NAT_IP=$(python3 -c "
import socket, struct, os
host, port = '$STUN_SERVER'.split(':')
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.settimeout(3)
tid = os.urandom(12)
msg = struct.pack('!HH', 0x0001, 0) + b'\x21\x12\xa4\x42' + tid
sock.sendto(msg, (host, int(port)))
data, _ = sock.recvfrom(1024)
i = 20
while i < len(data) - 4:
    atype, alen = struct.unpack('!HH', data[i:i+4])
    if atype in (0x0001, 0x8020, 0x0020) and alen >= 8:
        family = data[i+5]
        if family == 1:
            if atype in (0x0020, 0x8020):
                xport = struct.unpack('!H', data[i+6:i+8])[0] ^ 0x2112
                xip = struct.unpack('!I', data[i+8:i+12])[0] ^ 0x2112A442
                print(socket.inet_ntoa(struct.pack('!I', xip)))
            else:
                print(socket.inet_ntoa(data[i+8:i+12]))
            break
    i += 4 + alen
sock.close()
" 2>/dev/null) && break
  done
  if [ -n "$NAT_IP" ]; then
    echo "Asterisk NAT: detected external IP via STUN: $NAT_IP"
  fi
fi

if [ -z "$NAT_IP" ]; then
  NAT_IP=$(getent ahostsv4 host.docker.internal 2>/dev/null | head -1 | awk '{print $1}')
fi
if [ -z "$NAT_IP" ]; then
  NAT_IP=$(ip route 2>/dev/null | grep default | awk '{print $3}')
fi
if [ -z "$NAT_IP" ]; then
  echo "WARNING: Could not detect host IP."
  NAT_IP="0.0.0.0"
fi

echo "Asterisk NAT: external_media_address=$NAT_IP"

cat > /etc/asterisk/pjsip_transport.conf << EOF
; Auto-generated — do not edit
[transport-udp]
type=transport
protocol=udp
bind=0.0.0.0:5060
external_media_address=$NAT_IP
external_signaling_address=$NAT_IP
EOF

# Voiceworker trunk
if [ -n "$VOICEWORKER_HOST" ]; then
  CONTAINER_IP=$(hostname -i 2>/dev/null | tr ' ' '\n' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
  [ -z "$CONTAINER_IP" ] && CONTAINER_IP="$NAT_IP"
  VOICEWORKER_EXTEN=${VOICEWORKER_EXTEN:-2000}

  cat > /etc/asterisk/pjsip_voiceworker.conf << EOF
[voiceworker-aor]
type=aor
contact=sip:${VOICEWORKER_HOST}

[voiceworker-trunk]
type=endpoint
transport=transport-udp
context=from-voiceworker
disallow=all
allow=ulaw,alaw
aors=voiceworker-aor
media_address=${CONTAINER_IP}
EOF

  cat > /etc/asterisk/extensions_voiceworker.conf << EOF
[from-internal](+)
exten => ${VOICEWORKER_EXTEN},1,NoOp(Calling voiceworker at ${VOICEWORKER_HOST})
 same => n,Dial(PJSIP/\${EXTEN}@voiceworker-trunk,30)
 same => n,Hangup()
EOF
  echo "Voiceworker: extension $VOICEWORKER_EXTEN -> $VOICEWORKER_HOST"
else
  rm -f /etc/asterisk/pjsip_voiceworker.conf /etc/asterisk/extensions_voiceworker.conf
fi

# Sound files
SOUNDS_DATA="/data/sounds/en"
SOUNDS_DIR="/var/lib/asterisk/sounds/en"
SOUNDS_URL="https://downloads.asterisk.org/pub/telephony/sounds/asterisk-core-sounds-en-ulaw-current.tar.gz"
if [ ! -f "$SOUNDS_DATA/vm-intro.ulaw" ]; then
  echo "Downloading Asterisk core sound files..."
  mkdir -p "$SOUNDS_DATA"
  curl -sSLk "$SOUNDS_URL" | tar xz -C "$SOUNDS_DATA" || echo "WARNING: Failed to download sound files."
fi
mkdir -p /var/lib/asterisk/sounds
ln -sfn "$SOUNDS_DATA" "$SOUNDS_DIR"

# --- Start services ---

# Start Asterisk with a pseudo-TTY (prevents CPU spin from console without TTY)
script -q -c "/usr/sbin/asterisk -f" /dev/null > /dev/null 2>&1 &
ASTERISK_PID=$!

sleep 2

# Start xpbx web server
export XPBX_LISTEN_ADDR=:8080
export XPBX_DB_PATH=/data/asterisk-realtime.db
export ARI_HOST=localhost
export ARI_PORT=8088
export ARI_USER=xpbx
export ARI_PASSWORD=secret
export LOG_LEVEL=${LOG_LEVEL:-info}

/app/xpbx &
XPBX_PID=$!

echo "xpbx ready — SIP on :5060, Web UI on :8080"

# Shutdown handler
trap 'kill $ASTERISK_PID $XPBX_PID 2>/dev/null; wait' TERM INT

# Wait for either process to exit
wait $ASTERISK_PID $XPBX_PID
