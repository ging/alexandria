#!/bin/sh
# Entrypoint for alexandria-client in Kubernetes: discovers the active Hub DID over TLS and launches the sidecar.
# Uses the mounted local CA certificate when present so TLS verification succeeds against kind ingress-nginx.
set -eu

HUB_PUBLIC_URL="${HUB_PUBLIC_URL:-https://hub.127.0.0.1.nip.io:8443}"
CA_CERT="/etc/alexandria/tls/ca.crt"

CA_FLAG=""
CURL_CA=""
if [ -s "$CA_CERT" ]; then
    CA_FLAG="-hub-ca=$CA_CERT"
    CURL_CA="--cacert $CA_CERT"
fi

RESOLVED_DID="${HUB_DID:-}"
if [ -z "$RESOLVED_DID" ]; then
    attempt=0
    while [ "$attempt" -lt 60 ]; do
        # shellcheck disable=SC2086
        if html=$(curl -fsSL $CURL_CA "${HUB_PUBLIC_URL}/ui/" 2>/dev/null); then
            RESOLVED_DID=$(printf '%s' "$html" | sed -n 's/.*"hub_did":"\([^"]*\)".*/\1/p' | head -n 1)
            if [ -n "$RESOLVED_DID" ]; then
                break
            fi
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
fi

if [ -z "$RESOLVED_DID" ]; then
    echo "start-client.sh: failed to discover hub_did from ${HUB_PUBLIC_URL}/ui/ (set HUB_DID env var)" >&2
    exit 1
fi

set -- \
    -config /etc/alexandria/client.yaml \
    -state-dir /var/lib/alexandria/client \
    -db "$CLIENT_DB_URL" \
    -hub "$HUB_PUBLIC_URL" \
    -namespace "${CLIENT_NAMESPACE:-mobility}" \
    -hub-did "$RESOLVED_DID"

if [ -n "$CA_FLAG" ]; then
    set -- "$@" "$CA_FLAG"
fi

exec /usr/local/bin/alexandria-client "$@"
