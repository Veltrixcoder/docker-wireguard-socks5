#!/usr/bin/env bash
set -e

echo "[ENTRYPOINT] Starting WireGuard HTTP Proxy"

# Check if WireGuard config should be generated
if [[ -z "${WIREGUARD_INTERFACE_PRIVATE_KEY}" ]]; then
    echo "[ENTRYPOINT] Generating Cloudflare Warp configuration..."
    
    # Run warp binary to generate config
    WARP_OUTPUT=$(warp)
    
    # Parse the warp output to extract config values
    export WIREGUARD_INTERFACE_PRIVATE_KEY=$(echo "$WARP_OUTPUT" | grep "PrivateKey" | awk '{print $3}')
    export WIREGUARD_INTERFACE_ADDRESS=$(echo "$WARP_OUTPUT" | grep "Address" | awk '{print $3}')
    export WIREGUARD_PEER_PUBLIC_KEY=$(echo "$WARP_OUTPUT" | grep "PublicKey" | awk '{print $3}')
    export WIREGUARD_PEER_ENDPOINT=$(echo "$WARP_OUTPUT" | grep "Endpoint" | awk '{print $3}')
    export WIREGUARD_INTERFACE_DNS="${WIREGUARD_INTERFACE_DNS:-1.1.1.1}"
    
    echo "[ENTRYPOINT] Warp config generated successfully"
else
    echo "[ENTRYPOINT] Using provided WireGuard configuration"
fi

# Start the server (it handles WireGuard internally via netstack)
echo "[ENTRYPOINT] Starting HTTP proxy server..."
exec server "$@"
