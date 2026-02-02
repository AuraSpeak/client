// Package client contains the application-layer client that delegates networking to github.com/auraspeak/network/client.
// It handles client state, default packet handlers (e.g. ClientNeedsDisconnect), and exposes Run/Stop/Send/OnPacket.
package client

import (
	"context"

	"github.com/auraspeak/client/pkg/command"
	"github.com/auraspeak/client/pkg/state"
	netclient "github.com/auraspeak/network/client"
	"github.com/auraspeak/protocol"
	log "github.com/sirupsen/logrus"
)

// Client is the application-layer client: it wraps the network client and adds state and commands.
type Client struct {
	nc *netclient.Client

	Host string
	Port int

	state.ClientState

	OutCommandCh chan command.InternalCommand
}

// NewClient creates a new client for the given host and port.
func NewClient(Host string, Port int) *Client {
	cfg := netclient.ClientConfig{
		Host:               Host,
		Port:               Port,
		InsecureSkipVerify: false,
	}
	nc := netclient.NewClient(cfg)
	c := &Client{
		nc:           nc,
		Host:         Host,
		Port:         Port,
		OutCommandCh: make(chan command.InternalCommand, 10),
	}
	nc.OnPacket(protocol.PacketTypeClientNeedsDisconnect, func(packet *protocol.Packet, peer string) error {
		log.WithField("caller", "client").Infof("Received ClientNeedsDisconnect from server, reason: %s", string(packet.Payload))
		c.Stop()
		return nil
	})
	return c
}

// PacketHandler is the application-layer handler type (peer is always "" for client).
type PacketHandler func(packet *protocol.Packet) error

// OnPacket registers a handler for a packet type.
func (c *Client) OnPacket(packetType protocol.PacketType, handler PacketHandler) {
	c.nc.OnPacket(packetType, func(packet *protocol.Packet, peer string) error {
		return handler(packet)
	})
}

// Run starts the client and connects to the server. Blocks until Stop or disconnect.
func (c *Client) Run() error {
	c.SetRunningState(true)
	defer c.SetRunningState(false)
	return c.nc.Run(context.Background())
}

// Stop stops the client and closes the connection.
func (c *Client) Stop() {
	c.nc.Stop()
}

// Send sends raw bytes to the server.
func (c *Client) Send(msg []byte) error {
	return c.nc.Send(msg)
}

// NewDebugClient creates a new debug client with the given host, port, and ID.
// This function is only available in debug builds.
func NewDebugClient(Host string, Port int, ID int) *Client {
	log.Error("NewDebugClient is not implemented in release build")
	return nil
}
