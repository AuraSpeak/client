# Client

The Client package provides the application-layer client for the AuraSpeak Project. It delegates transport to `github.com/auraspeak/network/client` and adds state, commands and a default handler (e.g. `ClientNeedsDisconnect`). It exposes Run, Stop, Send and OnPacket. It utilizes the [AuraSpeak Protocol](https://github.com/AuraSpeak/protocol).

---

## Requirements

GO Version: 1.25.1

Go dependencies:
- `github.com/auraspeak/network` for local development replaced: `replace github.com/auraspeak/network => ../network`
- `github.com/auraspeak/protocol` for local development replaced: `replace github.com/auraspeak/protocol => ../protocol`

Indirect:
- `pion/dtls`
- `logrus`
- `testify`

---

## Structure

### Client

Application-layer client that wraps the network client. Holds `ClientState` and `OutCommandCh`. `NewClient(Host, Port)` creates a client and registers `ClientNeedsDisconnect` internally (calls `Stop()` on receipt).

### pkg/command

Defines `InternalCommand`, e.g. `CmdUpdateClientState`.

### pkg/state

`ClientState` (release build: `Running`; debug build has additional fields).

### set_state.go

State updates for the command channel.

---

## Quick start

`Client`
- Host
- Port

`Functions` / `Methods`
- `NewClient(host, port)` creates a new client
- `OnPacket(packetType, handler)` registers a handler; `PacketHandler` is `func(packet *protocol.Packet) error`
- `Run()` connects and blocks until Stop or disconnect
- `Send(msg []byte)` sends raw bytes to the server
- `Stop()` for a clean stop

Example usage: see `cmd/main.go` (stdin loop, signal handler for graceful shutdown).

---

## Testing

Run `go test ./...` to test. The client contains test hooks (e.g. in client_test).

---

## License

[License](./LICENSE)
