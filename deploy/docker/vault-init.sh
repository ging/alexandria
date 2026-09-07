#!/usr/bin/env bash
#
# Initialises the Vault in this stack, once, on its very first start.
#
# It prints five unseal keys and a root token, and they exist nowhere else: this
# script does not write them to disk, because the one place they must never be
# is next to the store they open. Put them somewhere out of band before you
# close the terminal — a password manager, five envelopes, whatever your
# operational practice is. Losing them means losing the node's private keys, and
# there is no recovery.
#
# After this, `bash vault-unseal.sh` on every restart of the Vault container.

set -euo pipefail

cd "$(dirname "$0")"
export COMPOSE_FILE="docker-compose.yaml"
export COMPOSE_PROFILES="vault"

if [[ ! -f .env ]]; then
  echo "no .env here. Copy .env.example and fill it in first." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

# The first "vault" is the compose service, the second the binary in it.
v() { docker compose exec -T -e VAULT_ADDR=http://127.0.0.1:8200 vault vault "$@"; }

if ! docker compose ps --status running --services | grep -qx vault; then
  echo "Vault is not running. Start it first:" >&2
  echo "    docker compose --profile vault up -d vault" >&2
  exit 1
fi

# `vault status` exits 2 when sealed and 0 when open, so its exit code cannot
# distinguish "not initialised" — read the field instead.
initialised="$(v status -format=json 2>/dev/null | sed -n 's/.*"initialized": *\([a-z]*\).*/\1/p' | head -n 1 || true)"

if [[ "$initialised" == "true" ]]; then
  cat >&2 <<'MSG'
This Vault is already initialised, and its unseal keys were printed once, when
it was. They cannot be printed again.

  bash vault-unseal.sh     if it is merely sealed
MSG
  exit 1
fi

echo "Initialising. The keys below are shown once and never again."
echo

init="$(v operator init -key-shares=5 -key-threshold=3 -format=json)"
printf '%s\n' "$init" | python3 -c '
import json, sys
d = json.load(sys.stdin)
print()
for i, k in enumerate(d["unseal_keys_b64"], 1):
    print(f"  unseal key {i}   {k}")
print()
print(f"  root token      {d['"'"'root_token'"'"']}")
print()
print("  Any three of the five keys unseal it. Store them apart from each")
print("  other and apart from this host.")
print()
'

echo "Now unseal it, with three of those keys:"
echo
echo "    bash vault-unseal.sh"
echo
echo "Then, still once, prepare it for the wallet:"
echo
echo "    VAULT_TOKEN=<root token> bash vault-configure.sh"
echo
