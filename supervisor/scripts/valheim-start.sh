#!/bin/bash
set -e

echo "Installing/updating Valheim server..."

# Install/update Valheim server (steamcmd location in cm2network/steamcmd image)
/home/steam/steamcmd/steamcmd.sh +force_install_dir /valheim +login anonymous +app_update 896660 validate +quit

# Set up environment
cd /valheim
export LD_LIBRARY_PATH=/valheim/linux64:$LD_LIBRARY_PATH
export SteamAppId=892970

echo "Starting Valheim server..."

# Run the server
exec /valheim/valheim_server.x86_64 \
    -name "${SERVER_NAME:-Valheim Server}" \
    -world "${WORLD_NAME:-Dedicated}" \
    -password "${SERVER_PASS:-}" \
    -port 2456 \
    -public 0 \
    -savedir /config
