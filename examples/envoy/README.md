# Example: Envoy as the L7 proxy, `alexandria-client` as validator

```text
counterparty ──▶ Envoy :LISTEN_PORT ─────────────────▶ connector
                   │  ▲
         ext_authz │  │ 200 (+ VHub-Verdict)  /  400 + DSP error
                   ▼  │
          alexandria-client :VALIDATOR_PORT (mode: validator)
```

Envoy owns the traffic. Before forwarding a request, its [`ext_authz`](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter) HTTP filter sends the method, path and body to `alexandria-client`, which answers without ever contacting the connector.

## Run

```bash
go run ./examples/envoy
```

It launches the lab with the client in validator mode, starts Envoy, replays F6 through Envoy and verifies the same verdict table as the proxy example. Envoy is taken from `PATH` when installed; otherwise it runs in Docker (`-image` selects the image, default `envoyproxy/envoy:v1.33-latest`). `-keep` leaves everything running.

## The check contract

A traffic listener in validator mode (`dsp-ingress`, `dsp-egress`, `mgmt-ingress`) expects the check to carry the **original method, path and body**. Rules still match on the listener, so each proxy listener asks the client listener of the same name.

| Decision | Validator answers | What Envoy does |
|:---|:---|:---|
| `forward` | `200`, empty body | Forwards to the connector. |
| `forward-notice` | `200` with `VHub-Verdict` | Forwards; `allowed_client_headers_on_success` copies the header onto the response to the sender. |
| `reject` | `400`/`422` with the §4.8 body and `VHub-Verdict` | Relays status, headers and body to the sender; the connector is never called. |

The parts of [`envoy.yaml`](envoy.yaml) that matter:

- `with_request_body` with `allow_partial_message: false`: the gates validate the body, so it must arrive whole. Requests larger than `max_request_bytes` are answered `413` by Envoy.
- `failure_mode_allow: false`: no decision, no forwarding.
- `allowed_client_headers_on_success` / `allowed_client_headers`: let `VHub-Verdict` (and the reject `Content-Type`) reach the sender.
- The route rewrite to `/dsp/` happens after the check, so rule paths stay the DSP paths (`/negotiations/{providerPid}/request`).

## Files

- [`client.yaml`](client.yaml): reference client configuration (`mode: validator`, no `upstreams`).
- [`envoy.yaml`](envoy.yaml): Envoy configuration with `${NAME}` placeholders; the header comment shows how to render it by hand.
- [`main.go`](main.go): the launcher.

## Limits of validator mode

The client never sees the connector's response: agreements are captured when the `ContractAgreementMessage` is allowed (not on the connector's `2xx`), and `validate_response` rules are not evaluated. Keep the validator listener reachable only by the proxy. See [`../README.md`](../README.md).
