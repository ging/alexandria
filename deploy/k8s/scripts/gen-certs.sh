#!/usr/bin/env bash
# Generates a local Root CA and SAN leaf certificate for *.127.0.0.1.nip.io and syncs Kubernetes TLS secrets.
# Reuses an existing Root CA in deploy/k8s/.certs/ so users only need to trust deploy/k8s/ca-root.crt once.
set -euo pipefail

K8S_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CERTS_DIR="${K8S_DIR}/.certs"
NAMESPACE="${K8S_NAMESPACE:-alexandria}"
CERTS_ONLY=0

while [ "$#" -gt 0 ]; do
    case "$1" in
        --certs-only)
            CERTS_ONLY=1
            shift
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        *)
            echo "Usage: $0 [--certs-only] [--namespace <ns>]" >&2
            exit 1
            ;;
    esac
done

command -v openssl >/dev/null 2>&1 || { echo "gen-certs.sh: openssl is required" >&2; exit 1; }

mkdir -p "$CERTS_DIR"

CA_KEY="${CERTS_DIR}/ca.key"
CA_CRT="${CERTS_DIR}/ca.crt"
TLS_KEY="${CERTS_DIR}/tls.key"
TLS_CSR="${CERTS_DIR}/tls.csr"
TLS_CRT="${CERTS_DIR}/tls.crt"
TLS_EXT="${CERTS_DIR}/tls.ext"
EXPORTED_CA="${K8S_DIR}/ca-root.crt"

# Generate the local Root CA once and reuse it across cluster recreations.
if [ ! -s "$CA_KEY" ] || [ ! -s "$CA_CRT" ]; then
    echo "Generating local Root CA in ${CERTS_DIR}..."
    openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \
        -keyout "$CA_KEY" \
        -out "$CA_CRT" \
        -subj "/CN=Alexandria Local Kind Root CA/O=Alexandria" \
        -addext "basicConstraints=critical,CA:TRUE" \
        -addext "keyUsage=critical,keyCertSign,cRLSign" >/dev/null 2>&1
    chmod 0600 "$CA_KEY"
fi

cp "$CA_CRT" "$EXPORTED_CA"
chmod 0644 "$EXPORTED_CA"

# Generate or refresh the leaf TLS certificate covering all nip.io hostnames used by the stack.
if [ ! -s "$TLS_KEY" ] || [ ! -s "$TLS_CRT" ] || ! openssl verify -CAfile "$CA_CRT" "$TLS_CRT" >/dev/null 2>&1; then
    echo "Generating TLS certificate for *.127.0.0.1.nip.io..."
    cat >"$TLS_EXT" <<'EOF'
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:hub.127.0.0.1.nip.io,DNS:auth.127.0.0.1.nip.io,DNS:client.127.0.0.1.nip.io,DNS:dsp.127.0.0.1.nip.io,DNS:*.127.0.0.1.nip.io,DNS:localhost,IP:127.0.0.1
EOF
    openssl req -newkey rsa:2048 -sha256 -nodes \
        -keyout "$TLS_KEY" \
        -out "$TLS_CSR" \
        -subj "/CN=hub.127.0.0.1.nip.io/O=Alexandria" >/dev/null 2>&1
    openssl x509 -req -in "$TLS_CSR" -CA "$CA_CRT" -CAkey "$CA_KEY" -CAcreateserial \
        -out "$TLS_CRT" -days 825 -sha256 -extfile "$TLS_EXT" >/dev/null 2>&1
    chmod 0600 "$TLS_KEY"
    rm -f "$TLS_CSR" "$TLS_EXT"
fi

if [ "$CERTS_ONLY" -eq 1 ]; then
    echo "Local CA exported to ${EXPORTED_CA}"
    exit 0
fi

command -v kubectl >/dev/null 2>&1 || { echo "gen-certs.sh: kubectl is required" >&2; exit 1; }

kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

kubectl -n "$NAMESPACE" create secret tls alexandria-tls \
    --cert="$TLS_CRT" \
    --key="$TLS_KEY" \
    --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NAMESPACE" create secret generic alexandria-ca \
    --from-file=ca.crt="$CA_CRT" \
    --dry-run=client -o yaml | kubectl apply -f -

echo "Synced Kubernetes TLS secrets (alexandria-tls, alexandria-ca) in namespace '${NAMESPACE}'."
echo "Local CA exported to ${EXPORTED_CA}"
