#!/bin/bash
# Tiny HTTP server that serves auriga-stats.sh output on port 9100
# Run as systemd user service: systemctl --user start auriga-stats-server

PORT="${1:-9100}"

while true; do
  RESPONSE=$(~/bin/auriga-stats.sh)
  CONTENT_LENGTH=${#RESPONSE}
  echo -e "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${CONTENT_LENGTH}\r\nAccess-Control-Allow-Origin: *\r\n\r\n${RESPONSE}" | nc -l -p "$PORT" -q 1 2>/dev/null || \
  echo -e "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: ${CONTENT_LENGTH}\r\nAccess-Control-Allow-Origin: *\r\n\r\n${RESPONSE}" | ncat -l -p "$PORT" --send-only 2>/dev/null
done
