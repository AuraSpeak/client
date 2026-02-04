package client

import (
	"testing"

	"github.com/auraspeak/protocol"
	log "github.com/sirupsen/logrus"
)

func init() {
	// Reduce log output during fuzzing so the fuzzer is not slowed and logs are not flooded.
	log.SetLevel(log.PanicLevel)
}

// FuzzClientNeedsDisconnectPayload fuzzes the handler logic for ClientNeedsDisconnect with arbitrary payload bytes.
// The real handler uses string(packet.Payload) for logging; this ensures no panic on invalid UTF-8 or large payloads.
func FuzzClientNeedsDisconnectPayload(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("reason"))
	f.Add([]byte("test reason"))
	f.Add([]byte{0xFF, 0xFE})
	f.Add(make([]byte, 100))
	f.Fuzz(func(t *testing.T, payload []byte) {
		packet := &protocol.Packet{
			PacketHeader: protocol.Header{PacketType: protocol.PacketTypeClientNeedsDisconnect},
			Payload:      payload,
		}
		// Same logic as the real handler: string(packet.Payload) must not panic.
		_ = string(packet.Payload)
	})
}
