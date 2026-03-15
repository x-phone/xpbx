#!/bin/sh
# All-in-one entrypoint: starts Asterisk + xpbx web server
set -e

# Run the Asterisk entrypoint (generates configs, starts Asterisk in background)
/scripts/entrypoint.sh -f &
ASTERISK_PID=$!

# Wait for Asterisk to be ready
sleep 2

# Start xpbx web server
export XPBX_LISTEN_ADDR=:8080
export XPBX_DB_PATH=/data/asterisk-realtime.db
export ARI_HOST=localhost
export ARI_PORT=8088
export ARI_USER=xpbx
export ARI_PASSWORD=secret
export LOG_LEVEL=${LOG_LEVEL:-info}

cd /app
./xpbx &
XPBX_PID=$!

# Wait for either process to exit
wait -n $ASTERISK_PID $XPBX_PID 2>/dev/null || true

# If one dies, kill the other
kill $ASTERISK_PID $XPBX_PID 2>/dev/null
wait
