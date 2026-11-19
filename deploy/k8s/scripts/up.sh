#!/usr/bin/env bash
# Bootstraps a local kind cluster with ingress-nginx, CoreDNS nip.io hairpinning, local TLS, and Alexandria via Kustomize.
# Mirrors deploy/docker (base stack by default, or --demo to also deploy alexandria-client and fake-connector).
set -euo pipefail

K8S_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "${K8S_DIR}/../.." && pwd)"

CLUSTER_NAME="${KIND_CLUSTER_NAME:-alexandria}"
NAMESPACE="${K8S_NAMESPACE:-alexandria}"
INGRESS_NGINX_MANIFEST="${INGRESS_NGINX_MANIFEST:-https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml}"

DEMO=0
SKIP_BUILD=0

while [ "$#" -gt 0 ]; do
    case "$1" in
        --demo)
            DEMO=1
            shift
            ;;
        --skip-build)
            SKIP_BUILD=1
            shift
            ;;
        --cluster)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        *)
            echo "Usage: $0 [--demo] [--skip-build] [--cluster <name>]" >&2
            exit 1
            ;;
    esac
done

for bin in docker kind kubectl openssl; do
    command -v "$bin" >/dev/null 2>&1 || { echo "up.sh: required command not found: ${bin}" >&2; exit 1; }
done

# 1. Create the kind cluster if it does not exist yet.
if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
    echo "==> Creating kind cluster '${CLUSTER_NAME}'..."
    kind create cluster --name "$CLUSTER_NAME" --config "${K8S_DIR}/kind-config.yaml"
else
    echo "==> Reusing existing kind cluster '${CLUSTER_NAME}'."
fi

kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

# 2. Install ingress-nginx (official kind provider manifest) and wait for readiness.
echo "==> Ensuring ingress-nginx controller is installed and ready..."
kubectl apply -f "$INGRESS_NGINX_MANIFEST" >/dev/null
kubectl -n ingress-nginx wait --for=condition=complete job --all --timeout=120s >/dev/null 2>&1 || true
kubectl -n ingress-nginx wait --for=condition=ready pod \
    --selector=app.kubernetes.io/component=controller \
    --timeout=180s

# 3. Expose ingress-nginx internally on port 8443 and rewrite *.127.0.0.1.nip.io in CoreDNS for pod hairpinning.
echo "==> Configuring in-cluster DNS hairpinning for *.127.0.0.1.nip.io..."
kubectl apply -f - >/dev/null <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: alexandria-ingress-internal
  namespace: ingress-nginx
  labels:
    app.kubernetes.io/part-of: alexandria
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/component: controller
    app.kubernetes.io/name: ingress-nginx
  ports:
    - name: http
      port: 80
      targetPort: http
    - name: https
      port: 443
      targetPort: https
    - name: https-alt
      port: 8443
      targetPort: https
EOF

CURRENT_COREFILE="$(kubectl -n kube-system get configmap coredns -o jsonpath='{.data.Corefile}')"
if ! printf '%s' "$CURRENT_COREFILE" | grep -q 'alexandria-ingress-internal.ingress-nginx.svc.cluster.local'; then
    TMP_COREFILE="$(mktemp)"
    trap 'rm -f "$TMP_COREFILE"' EXIT
    printf '%s\n' "$CURRENT_COREFILE" | awk '
        /^[[:space:]]*ready[[:space:]]*$/ && !inserted {
            print
            print "    rewrite name exact hub.127.0.0.1.nip.io alexandria-ingress-internal.ingress-nginx.svc.cluster.local answer auto"
            print "    rewrite name exact auth.127.0.0.1.nip.io alexandria-ingress-internal.ingress-nginx.svc.cluster.local answer auto"
            print "    rewrite name exact client.127.0.0.1.nip.io alexandria-ingress-internal.ingress-nginx.svc.cluster.local answer auto"
            print "    rewrite name exact dsp.127.0.0.1.nip.io alexandria-ingress-internal.ingress-nginx.svc.cluster.local answer auto"
            inserted = 1
            next
        }
        { print }
    ' >"$TMP_COREFILE"
    kubectl -n kube-system create configmap coredns \
        --from-file=Corefile="$TMP_COREFILE" \
        --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    kubectl -n kube-system rollout restart deployment/coredns >/dev/null
    kubectl -n kube-system rollout status deployment/coredns --timeout=90s >/dev/null
fi

# 4. Generate/sync local CA and TLS secrets.
echo "==> Syncing local CA and TLS secrets..."
"${K8S_DIR}/scripts/gen-certs.sh" --namespace "$NAMESPACE"

# 5. Build local Docker images and load them into the kind cluster.
if [ "$SKIP_BUILD" -eq 0 ]; then
    echo "==> Building alexandria-server:local..."
    docker build -f "${REPO_ROOT}/Dockerfile" --target server -t alexandria-server:local "$REPO_ROOT"
    kind load docker-image --name "$CLUSTER_NAME" alexandria-server:local

    if [ "$DEMO" -eq 1 ]; then
        echo "==> Building alexandria-client:local and alexandria-fake-connector:local..."
        docker build -f "${REPO_ROOT}/Dockerfile" --target client -t alexandria-client:local "$REPO_ROOT"
        docker build -f "${REPO_ROOT}/Dockerfile" --target fake-connector -t alexandria-fake-connector:local "$REPO_ROOT"
        kind load docker-image --name "$CLUSTER_NAME" alexandria-client:local alexandria-fake-connector:local
    fi
fi

# 6. Apply Kustomize manifests (base or overlays/demo).
TARGET_DIR="${K8S_DIR}/base"
if [ "$DEMO" -eq 1 ]; then
    TARGET_DIR="${K8S_DIR}/overlays/demo"
fi

echo "==> Applying Kustomize manifests from ${TARGET_DIR}..."
attempt=0
until kubectl apply -k "$TARGET_DIR"; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 10 ]; then
        echo "up.sh: kubectl apply -k failed after ${attempt} attempts" >&2
        exit 1
    fi
    echo "Waiting for ingress-nginx admission webhook..."
    sleep 3
done

# 7. Wait for deployments to become ready.
echo "==> Waiting for deployments in namespace '${NAMESPACE}'..."
kubectl -n "$NAMESPACE" rollout status deployment/postgres --timeout=180s
kubectl -n "$NAMESPACE" rollout status deployment/keycloak --timeout=300s
kubectl -n "$NAMESPACE" rollout status deployment/alexandria-server --timeout=180s
if [ "$DEMO" -eq 1 ]; then
    kubectl -n "$NAMESPACE" rollout status deployment/fake-connector --timeout=120s
    kubectl -n "$NAMESPACE" rollout status deployment/alexandria-client --timeout=180s
fi

echo ""
echo "Alexandria is running on kind cluster '${CLUSTER_NAME}'!"
echo "  Hub API & Console : https://hub.127.0.0.1.nip.io:8443/ui/"
echo "  Keycloak Issuer   : https://auth.127.0.0.1.nip.io:8443/realms/alexandria"
echo "  Keycloak Admin    : https://auth.127.0.0.1.nip.io:8443/admin/ (admin / admin)"
if [ "$DEMO" -eq 1 ]; then
    echo "  Client Console    : https://client.127.0.0.1.nip.io:8443/ui/ (or http://127.0.0.1:9090/ui/)"
    echo "  Client DSP Proxy  : https://dsp.127.0.0.1.nip.io:8443 (or http://127.0.0.1:9080)"
fi
echo ""
echo "Local CA certificate: ${K8S_DIR}/ca-root.crt"
echo "  curl: export CURL_CA_BUNDLE=\"${K8S_DIR}/ca-root.crt\""
