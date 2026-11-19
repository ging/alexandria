#!/usr/bin/env bash
# Removes Alexandria Kubernetes resources or deletes the entire kind cluster (--cluster).
# Mirrors task docker:down while letting users keep or destroy the local kind cluster.
set -euo pipefail

CLUSTER_NAME="${KIND_CLUSTER_NAME:-alexandria}"
NAMESPACE="${K8S_NAMESPACE:-alexandria}"
DELETE_CLUSTER=0

while [ "$#" -gt 0 ]; do
    case "$1" in
        --cluster)
            DELETE_CLUSTER=1
            shift
            ;;
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        *)
            echo "Usage: $0 [--cluster] [--cluster-name <name>]" >&2
            exit 1
            ;;
    esac
done

if [ "$DELETE_CLUSTER" -eq 1 ]; then
    command -v kind >/dev/null 2>&1 || { echo "down.sh: kind is required" >&2; exit 1; }
    echo "==> Deleting kind cluster '${CLUSTER_NAME}'..."
    kind delete cluster --name "$CLUSTER_NAME"
    exit 0
fi

command -v kubectl >/dev/null 2>&1 || { echo "down.sh: kubectl is required" >&2; exit 1; }

if kubectl config get-contexts -o name 2>/dev/null | grep -qx "kind-${CLUSTER_NAME}"; then
    kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
    echo "==> Deleting namespace '${NAMESPACE}' from kind cluster '${CLUSTER_NAME}'..."
    kubectl delete namespace "$NAMESPACE" --ignore-not-found
else
    echo "Kind context 'kind-${CLUSTER_NAME}' not found; nothing to delete."
fi
