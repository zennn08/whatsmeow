# whatsmeow-wam fork patches

This is a fork of [whatsmeow](https://github.com/tulir/whatsmeow) carrying two
small, additive patches needed by
[`github.com/zennn08/whatsmeow-wam`](https://github.com/zennn08/whatsmeow-wam)
(WhatsApp Web WAM / `w:stats` telemetry). Nothing upstream is removed or changed
in behaviour; both patches are inert unless used.

## 1. `Client.RawNodeHandler` (`rawnode.go`, `client.go`)

An optional hook fired for every raw binary node sent and received, before normal
processing — the transport-level visibility WA Web has:

```go
type RawNodeEvent struct {
    Node     *waBinary.Node // the raw node (do not mutate received nodes)
    Outgoing bool           // true for sent nodes
    Handled  bool           // received nodes: whether whatsmeow has a handler (false ≈ unhandled stanza)
}

// on Client:
RawNodeHandler func(evt RawNodeEvent)
```

Invoked in `handleFrame` (incoming) and `sendNodeAndGetData` (outgoing). Nil by
default (zero overhead). whatsmeow-wam uses it to derive the outbound and
raw-stanza WAM events the typed event API can't express.

## 2. Exported info-query API (`request.go`)

Type aliases + constants so external packages can build an IQ and send it via the
existing `cli.DangerousInternals().SendIQ`, which waits for the ack:

```go
type (
    InfoQuery     = infoQuery
    InfoQueryType = infoQueryType
)
const (
    IQGet InfoQueryType = iqGet
    IQSet InfoQueryType = iqSet
)
```

The `infoQuery` struct's fields were already exported; these aliases just make the
unexported type nameable from outside. No mass rename of the internal API.

## Keeping in sync with upstream

The module path is still `go.mau.fi/whatsmeow`, so upstream rebases apply cleanly.
The patches live in `rawnode.go` (new file) and a handful of lines in `client.go`
and `request.go` — grep for `RawNodeHandler` and `InfoQuery` to find them.
