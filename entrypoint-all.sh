#!/bin/bash
# All-in-one entrypoint: Asterisk + xpbx web server
set -e

# --- Generate Asterisk configs (NAT, transport, voiceworker) ---
# Reuse the same config generation logic from the Asterisk entrypoint.
# Source it as a function library — we override exec to prevent it from launching Asterisk.
exec() { :; }
. /scripts/entrypoint.sh
unset -f exec

# --- Start xpbx first to create/seed the database ---
# Asterisk needs the SQLite tables to exist before it starts (Realtime lookups).
export XPBX_LISTEN_ADDR=${XPBX_LISTEN_ADDR:-:8080}
export XPBX_DB_PATH=${XPBX_DB_PATH:-/data/asterisk-realtime.db}
export ARI_HOST=localhost
export ARI_PORT=8088
export ARI_USER=${ARI_USER:-xpbx}
export ARI_PASSWORD=${ARI_PASSWORD:-secret}
export LOG_LEVEL=${LOG_LEVEL:-info}

/app/xpbx &
XPBX_PID=$!

# Wait for xpbx to create tables and seed data
sleep 2

# --- Start Asterisk ---
# Asterisk's console mode spins at 100% CPU without a TTY (tight select() loop on stdin).
# Use `script` to provide a pseudo-TTY, suppress all output (logs go through logger.conf).
script -qfc "/usr/sbin/asterisk -fp" /dev/null > /dev/null 2>&1 &
ASTERISK_PID=$!

# Wait for Asterisk to be ready, then reload PJSIP to pick up seeded endpoints
for i in $(seq 1 30); do
  /usr/sbin/asterisk -rx "core show version" > /dev/null 2>&1 && break
  sleep 1
done
/usr/sbin/asterisk -rx "module reload res_pjsip.so" > /dev/null 2>&1 || true

echo "xpbx ready — SIP on :5060, Web UI on :8080"

# Shutdown handler
trap 'kill $ASTERISK_PID $XPBX_PID 2>/dev/null; wait' TERM INT

# Wait for either process to exit, then stop the other
wait -n $ASTERISK_PID $XPBX_PID 2>/dev/null || true
kill $ASTERISK_PID $XPBX_PID 2>/dev/null || true
wait 2>/dev/null || true
exit 1
