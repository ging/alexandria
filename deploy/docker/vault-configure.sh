#!/usr/bin/env bash
#
# Prepares an unsealed Vault for the wallet: the secrets engine it writes to, a
# policy that reaches only that, a token scoped to that policy, and the database
# credential the wallet reads before it looks at Vault at all.
#
# Run once, after vault-init.sh and vault-unseal.sh, with the root token:
#
#   VAULT_TOKEN=<root token> bash vault-configure.sh
#
# It writes the wallet's token into .env and does not use the root token for
# anything else. The root token should then be stored out of band with the
# unseal keys, or revoked and re-created when it is next needed.

set -euo pipefail

cd "$(dirname "$0")"
export COMPOSE_FILE="docker-compose.yaml"
export COMPOSE_PROFILES="vault"

if [[ -z "${VAULT_TOKEN:-}" ]]; then
  echo "VAULT_TOKEN is not set. Pass the root token from vault-init.sh:" >&2
  echo "    VAULT_TOKEN=<root token> bash vault-configure.sh" >&2
  exit 1
fi

# Held across the source below. .env carries a VAULT_TOKEN of its own once this
# script has run before — the wallet's scoped one — and sourcing it would
# silently replace the root token the operator just passed, so the second run
# would fail on permissions it is supposed to be granting.
ROOT_TOKEN="$VAULT_TOKEN"

set -a
# shellcheck disable=SC1091
source .env
set +a

VAULT_TOKEN="$ROOT_TOKEN"
MOUNT="${VAULT_MOUNT:-secret}"

v() {
  docker compose exec -T \
    -e VAULT_ADDR=http://127.0.0.1:8200 \
    -e VAULT_TOKEN="$VAULT_TOKEN" \
    vault vault "$@"
}

if ! v status >/dev/null 2>&1; then
  echo "Vault is sealed or not answering. Run vault-unseal.sh first." >&2
  exit 1
fi

# ===== The secrets engine ====================================================
# KV version 2, which is what the wallet's client speaks: it addresses
# <mount>/data/<path> rather than <mount>/<path>.

if v secrets list -format=json | grep -q "\"$MOUNT/\""; then
  echo "mount      $MOUNT/ exists"
else
  v secrets enable -path="$MOUNT" -version=2 kv
  echo "mount      $MOUNT/ created"
fi

# ===== The policy ============================================================
# Scoped to that mount and nothing else, so a wallet that is compromised cannot
# read the rest of the Vault. It needs the data paths to store keys and the
# metadata paths because listing and deleting go through them.

v policy write fafnir - <<POLICY
path "$MOUNT/data/*" {
  capabilities = ["create", "read", "update", "patch", "delete", "list"]
}

path "$MOUNT/metadata/*" {
  capabilities = ["read", "list", "delete"]
}

# The client reads this to find out which KV version the mount is, before it
# reads anything else. Without it every call fails with a 403 on sys/mounts and
# nothing says which permission is missing.
path "sys/mounts" {
  capabilities = ["read"]
}
POLICY
echo "policy     fafnir"

# ===== The database credential ===============================================
# Read by the wallet from Vault. It also reads it from a file, which fafnir-init
# writes — that is a quirk of the wallet, not a choice here; see ../README.md.

v kv put "$MOUNT/db.json" \
  user="${FAFNIR_DB_USER:-fafnir}" \
  password="${FAFNIR_DB_PASSWORD:?set FAFNIR_DB_PASSWORD in .env}" \
  name="${FAFNIR_DB_NAME:-fafnir}" >/dev/null
echo "secret     $MOUNT/db.json"

# ===== The wallet's token ====================================================
# Periodic rather than fixed-TTL: it renews itself for as long as it is used and
# expires when it is not, which is what a long-running service wants. Renewal is
# the client's job, so an outage longer than the period means a new token —
# noted in the README rather than papered over.

token="$(v token create -policy=fafnir -period=768h -field=token)"

set_var() {
  local key="$1" value="$2"

  if grep -q "^${key}=" .env; then
    # A temporary file and a move rather than sed -i: the in-place flag takes an
    # argument on BSD sed and none on GNU, and this runs on both.
    sed "s|^${key}=.*|${key}=${value}|" .env > .env.tmp && mv .env.tmp .env
  else
    printf '%s=%s\n' "$key" "$value" >> .env
  fi
}

set_var VAULT_TOKEN "$token"
set_var VAULT_MOUNT "$MOUNT"
set_var FAFNIR_CONFIG ./fafnir/wallet-vault.yaml

echo "token      written to .env, scoped to the fafnir policy"
echo
echo "  The wallet is not using Vault until it restarts on the new"
echo "  configuration:"
echo
echo "      docker compose up -d --force-recreate fafnir-wallet"
echo
echo "  A wallet that already had a file-backed identity does not carry it"
echo "  over. Give it one in Vault:"
echo
echo "      bash ../../scripts/fafnir-bootstrap.sh"
echo
