# Example: `alexandria-client` as the L7 proxy

```text
counterparty ──▶ alexandria-client (dsp-ingress) ──▶ connector
```

The client is the only component in front of the connector: it decides on each message and forwards it byte for byte to `upstreams.dsp`. This is `mode: proxy`, the default.

## Run

```bash
go run ./examples/proxy
```

It launches the lab, replays F6 through the `dsp-ingress` listener and verifies the verdict table. With `-keep` the lab stays up so you can send requests yourself to the printed ingress URL:

```bash
curl -i -X POST "$INGRESS/negotiations/urn:uuid:p-1/request" -H 'Content-Type: application/json' -d '{"@type":"ContractRequestMessage","providerPid":"urn:uuid:p-1","consumerPid":"urn:uuid:c-1","action":"odrl:use"}'
```

## Files

- [`client.yaml`](client.yaml): reference configuration for this topology.
- [`main.go`](main.go): the launcher.

## What you get

| Decision | What the sender receives |
|:---|:---|
| `forward` | The connector's response, unchanged. |
| `forward-notice` | The connector's response plus the `VHub-Verdict` header. |
| `reject` | `400` with a DSP error body (or a problem document) and `VHub-Verdict`; the connector is never called. |
