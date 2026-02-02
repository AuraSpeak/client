//go:build debug
// +build debug

package client

import (
	"context"
	"strconv"
	"sync/atomic"

	"github.com/auraspeak/client/pkg/command"
	"github.com/auraspeak/client/pkg/state"
	netclient "github.com/auraspeak/network/client"
	"github.com/auraspeak/protocol"
	log "github.com/sirupsen/logrus"
)

// NewDebugClient creates a new debug client with the given host, port, and ID.
// This function is only available in debug builds.
func NewDebugClient(Host string, Port int, ID int) *Client {
	c := &Client{
		Host:         Host,
		Port:         Port,
		ClientState:  state.ClientState{ID: ID},
		OutCommandCh: make(chan command.InternalCommand, 10),
	}
	cfg := netclient.ClientConfig{
		Host:               Host,
		Port:               Port,
		InsecureSkipVerify: false,
		OnConnect:          func() { c.debugHello() },
	}
	c.nc = netclient.NewClient(cfg)
	c.nc.OnPacket(protocol.PacketTypeClientNeedsDisconnect, func(packet *protocol.Packet, peer string) error {
		log.WithField("caller", "client").Infof("Received ClientNeedsDisconnect from server, reason: %s", string(packet.Payload))
		c.Stop()
		return nil
	})
	return c
}

// SetRunningState updates the running state of the client atomically and notifies the web server (debug builds only).
func (c *Client) SetRunningState(running bool) {
	var v int32
	if running {
		v = 1
	}
	atomic.StoreInt32(&c.ClientState.Running, v)

	select {
	case <-context.Background().Done():
		return
	case c.OutCommandCh <- command.CmdUpdateClientState:
	default:
	}
}

func (c *Client) debugHello() {
	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte(strconv.Itoa(c.ClientState.ID)),
	}
	log.WithField("caller", "client").Infof("Sending debug hello packet, ID: %d", c.ClientState.ID)
	c.Send(packet.Encode())
}
