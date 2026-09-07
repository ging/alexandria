# Deploying with Docker

A whole node on one machine: the service, its database, the identity provider it
authenticates through, and the proxy that terminates TLS for both. Nothing here
is reachable from outside except through Caddy on ports 80 and 443.

This is not `docker-compose.dev.yaml` with the values changed. That file runs
the node on the host, against a certificate authority the project generates for
itself, with passwords that are committed on purpose. None of that survives
contact with a deployment, so the two files stay separate.

## What you need first

- A host with Docker and the compose plugin.
- Two DNS names pointing at it — one for the node, one for the identity
  provider. They must resolve publicly **before** the first start: Caddy issues
  from Let's Encrypt, which validates by connecting back on port 80.
- Ports 80 and 443 open.

## Setup

```sh
cd deploy/docker
cp .env.example .env
$EDITOR .env            # domains, passwords, master key — every field is commented
```

The random values:

```sh
openssl rand -hex 32    # POSTGRES_PASSWORD, ZITADEL_DB_PASSWORD, session key
openssl rand -hex 16    # ZITADEL_MASTERKEY — exactly 32 characters
```

Then bring it up. This is a three-step dance, and it is unavoidable: Zitadel
generates the node's client id — it cannot be chosen — so the node has no
credentials to start with until the provider it authenticates against exists.

```sh
docker compose up -d    # the node starts without OAuth credentials and says so
bash bootstrap.sh       # registers the node in Zitadel, writes them into .env
docker compose up -d    # again, now that they exist
```

`bootstrap.sh` is idempotent. Run it again after changing a domain, or after
somebody deletes the application in the console, and it converges on the same
state without disturbing the session key.

## Afterwards

The node is at `https://$ALEXANDRIA_DOMAIN`, the Zitadel console at
`https://$AUTH_DOMAIN/ui/console`. Log into the console once with the
administrator from `.env` and change the password — Zitadel will insist.

```sh
docker compose logs -f alexandria
docker compose ps
```

## Upgrading

```sh
$EDITOR .env            # bump ALEXANDRIA_IMAGE to the new tag
docker compose pull alexandria
docker compose up -d alexandria
```

Pin a tag rather than tracking `:latest`. Tags are published by
`.github/workflows/release.yaml` on every `v*` git tag, and `:latest` moving
underneath a `docker compose pull` is a change to how every login behaves.

The same applies to `ZITADEL_VERSION`, more so: read its release notes before
moving it. It owns every account in the deployment.

## What must be backed up

- The `fafnir-secrets` volume, before anything else — it holds the node's
  private keys, one file per key. With Vault on, it is the `vault-data` volume
  instead, and the unseal keys are then just as essential and must be kept
  somewhere else entirely: a backup of `vault-data` next to its unseal keys is
  a backup of the plaintext. Losing it does not lose data; it loses the
  node's identity, and every credential ever issued to that identity with it.
  Note that this is the volume to keep, not `fafnir-data`: the wallet's Postgres
  holds only key *metadata* and the DID records. Losing the keys while keeping
  that database is the worst of the two outcomes — the wallet goes on reporting
  a default DID it can no longer sign with.
- The `fafnir-data` volume — the wallet's DID records.
- The `postgres-data` volume — the node's own data.
- The `zitadel-data` volume — every user, project and application.
- `.env`, and `ZITADEL_MASTERKEY` above all. It encrypts everything Zitadel
  stores, and there is no recovery from losing it: the database becomes
  unreadable with it gone. `caddy-data` is worth keeping too — losing it means
  re-issuing certificates, which Let's Encrypt rate-limits.

## What is deliberately not published

Postgres, Zitadel's database, and the node's internal listener — `/metrics` and
`/debug/pprof` on port 2112 — are reachable on the compose network and nowhere
else. Zitadel's own port is published on `127.0.0.1` only, so `bootstrap.sh` can
reach the management API from a shell on the host without waiting for a
certificate. Scrape metrics by joining the network, not by opening a port.

## The wallet

The node's identity. It holds the private key and does the signing — alexandria
never sees key material — and the node comes up reporting itself not ready
until it answers.

It is in this stack, as `fafnir-wallet`, with a database of its own. It
publishes no port and Caddy does not proxy it: the node is the only thing that
talks to it. Its configuration is [fafnir/wallet.yaml](fafnir/wallet.yaml).

To use a wallet that runs somewhere else instead, set `WALLET_HOST` and
`WALLET_PORT` in `.env`. Nothing in the compose file assumes it is the bundled
one.

### Where its keys live

By default `is_vault_real` is false and the wallet keeps private keys as files
on the `fafnir-secrets` volume, one per key id. Its database credential is a
file there too, written by the `fafnir-init` container from
`FAFNIR_DB_PASSWORD`, so that password exists in `.env` and nowhere else.

To keep the keys in Vault instead, see below.

## Vault

Optional, and off as written. Turning it on moves the wallet's private keys out
of a file on a volume and into a Vault in this stack.

**It costs a manual step on every restart**, and that is deliberate. A Vault
with file storage seals itself whenever it starts and needs three of its five
unseal keys to open again. The alternatives are auto-unseal against a cloud KMS,
which a single-host stack cannot assume, or leaving the unseal keys on the host
beside the data they open — which would make the whole exercise decorative. So a
reboot of this host is a human intervention.

While Vault is sealed the wallet does not start: Vault reports itself unhealthy
and `fafnir-setup` waits on it rather than failing in a loop. The node comes up
and reports itself not ready, which is exactly what it does for any wallet it
cannot reach.

### Setting it up, once

```sh
docker compose --profile vault up -d vault
bash vault-init.sh        # prints five unseal keys and a root token
bash vault-unseal.sh      # three of those keys
VAULT_TOKEN=<root token> bash vault-configure.sh
docker compose up -d --force-recreate fafnir-wallet
bash ../../scripts/fafnir-bootstrap.sh
```

`vault-init.sh` prints the unseal keys and the root token **once**, and writes
them nowhere: the one place they must never be is beside the store they open.
Put them somewhere out of band before closing the terminal. Losing them loses
the node's private keys, and there is no recovery.

`vault-configure.sh` creates the KV mount, a policy scoped to it, a periodic
token for the wallet under that policy, and the database credential. It writes
the wallet's token — not the root one — into `.env`, and switches
`FAFNIR_CONFIG` to the Vault-backed wallet configuration.

A wallet that already had a file-backed identity does not carry it over: the
keys are in the old volume, not in Vault. `fafnir-bootstrap.sh` gives it a new
one, which is a new DID.

### After every restart

```sh
bash vault-unseal.sh
```

It prompts for three keys without echoing them, and starts the wallet once Vault
is open.

### What this does and does not buy

It buys: the keys are no longer files an operator can `docker cp` out, they are
encrypted at rest under the unseal keys, and access to them is a token scoped to
one mount with an audit trail behind it.

It does not buy: protection from someone who already has root on this host while
Vault is unsealed — the token is in `.env` and the keys are in memory. On one
machine that is the honest limit.

Vault listens on plain HTTP and publishes no port: it is reached by the wallet
on the compose network and by you through `docker compose exec`, the same trust
boundary as the Postgres beside it. Publishing that port means giving it TLS
first.
