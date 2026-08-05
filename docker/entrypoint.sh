#!/bin/sh
set -e

# Seed a clean config/providers file into the mounted volume on first boot
# only. Never overwrites what's already there — a restart must not clobber
# a token, provider keys, or any other setting the previous run persisted.
mkdir -p /memo/data /memo/config
if [ ! -f /memo/config/config.yaml ]; then
    cp /app/config.yaml.example /memo/config/config.yaml
fi
if [ ! -f /memo/data/providers.json ]; then
    cp /app/providers.example.json /memo/data/providers.json
fi

# --lan binds 0.0.0.0 (127.0.0.1 is unreachable through Docker's own port
# mapping) and requires the X-Memo-Token header on every request — the token
# is generated on first boot, persisted to config.yaml, and printed below on
# every boot after. `docker logs <container>` is how you read it.
exec /app/memo --headless --port 8090 --lan
