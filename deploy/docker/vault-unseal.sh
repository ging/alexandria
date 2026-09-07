#!/usr/bin/env bash
#
# Unseals the Vault in this stack. Needed on every start of that container —
# after a reboot, after `docker compose up`, after an image bump.
#
# This is the manual step the deployment accepts on purpose: the alternative is
# auto-unseal against a cloud KMS, which this single-host stack cannot assume,
# or keeping the unseal keys on the host beside the data they open, which would
# make the whole exercise decorative.
#
# Until it runs, the wallet does not start: Vault reports itself unhealthy and
# fafnir-setup waits on it rather than failing.

set -euo pipefail

cd "$(dirname "$0")"
export COMPOSE_FILE="docker-compose.yaml"
export COMPOSE_PROFILES="vault"

# stdin is closed on every call: `docker compose exec` drains it even with -T,
# and this script reads the unseal keys from it one at a time. Without this the
# first exec swallows the rest and the loop reads nothing.
v() { docker compose exec -T -e VAULT_ADDR=http://127.0.0.1:8200 vault vault "$@" </dev/null; }

status="$(v status -format=json 2>/dev/null || true)"

if [[ -z "$status" ]]; then
  echo "Vault is not answering. Start it first:" >&2
  echo "    docker compose --profile vault up -d vault" >&2
  exit 1
fi

if printf '%s' "$status" | grep -q '"sealed": *false'; then
  echo "already unsealed"
  exit 0
fi

threshold="$(printf '%s' "$status" | sed -n 's/.*"t": *\([0-9]*\).*/\1/p' | head -n 1)"
threshold="${threshold:-3}"

echo "Sealed. $threshold of the unseal keys will open it."
echo

for i in $(seq 1 "$threshold"); do
  # -s so the key is not echoed, and read from the terminal rather than an
  # argument: an unseal key in the shell history is an unseal key on disk.
  read -rsp "  unseal key $i of $threshold: " key
  echo

  if [[ -z "$key" ]]; then
    echo "empty, stopping" >&2
    exit 1
  fi

  out="$(v operator unseal -format=json "$key" 2>&1)" || {
    echo
    echo "that key was not accepted" >&2
    exit 1
  }

  if printf '%s' "$out" | grep -q '"sealed": *false'; then
    echo
    echo "unsealed"

    # Everything downstream was waiting on the healthcheck, which passes now —
    # but only once Vault has been prepared for the wallet. On the very first
    # unseal it has not been, and starting the wallet here would just fail it
    # against a Vault with no mount, no policy and no token.
    if [[ -f .env ]] && grep -q '^VAULT_TOKEN=.\+' .env; then
      echo "starting the wallet"
      docker compose up -d fafnir-wallet
    else
      echo
      echo "  Vault is open but not yet prepared for the wallet. Once, with the"
      echo "  root token from vault-init.sh:"
      echo
      echo "      VAULT_TOKEN=<root token> bash vault-configure.sh"
      echo
    fi

    exit 0
  fi
done

echo "still sealed" >&2
exit 1
