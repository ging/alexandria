# Examples: two ways to deploy `alexandria-client`

`alexandria-client` decides on every DSP message with local state only. *Who forwards the message* is a deployment choice, set with `mode` in the client configuration:

| | [`proxy/`](proxy) | [`envoy/`](envoy) |
|:---|:---|:---|
| `mode` | `proxy` (default) | `validator` |
| L7 proxy in front of the connector | `alexandria-client` | Envoy (or any proxy with an HTTP external-authorization hook) |
| Role of `alexandria-client` | Decides **and** forwards | Only decides; never touches the connector |
| `upstreams` | Required | Not used |
| Traffic listeners | Reverse / forward proxies | Check endpoints called by the proxy |

```text
proxy/    counterparty ──▶ alexandria-client ──▶ connector

envoy/    counterparty ──▶ Envoy ───────────────▶ connector
                             │  ▲
                   ext_authz ▼  │ 200 / reject
                       alexandria-client
```

Both examples start the same laboratory (hub, `fake-idp`, `fake-connector`, client), replay the F6 scenario through the topology and check the same 29-row verdict table: the decisions are identical, only the forwarder changes.

```bash
go run ./examples/proxy
```

```bash
go run ./examples/envoy
```

Common flags: `-keep` leaves the lab running and prints its endpoints, `-v` prints the logs of every process, `-bin-dir` points to prebuilt binaries (they are built on the fly when missing).

## What changes in validator mode

The decision (rules, gates, classification, `VHub-Verdict`, reject bodies, events, metrics, admin API) is the same code path. These points differ because the client no longer sees the response of the connector:

- **Agreement capture** happens when a `ContractAgreementMessage` is *allowed*, not after the connector answers `2xx`.
- **`validate_response`** rules (catalog responses, passive audit) are not evaluated.
- **Byte-identical forwarding** (`REQ-SYS-003`, `REQ-CLI-014`) is the proxy's job: Envoy adds its own headers (`x-request-id`, `x-forwarded-proto`, …).
- **Availability** is a proxy setting: the Envoy example fails closed (`failure_mode_allow: false`) when the validator is down.
