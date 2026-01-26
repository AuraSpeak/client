// Package Client contains the core networking for the Client
// It is responsible for sending and receiving packets to the Server
// The implementation is based on the UDP protocol
// It Handles Client States, Starts and Runs the Client
// Add callbacks to the Client to handle incoming packets
// It may contain some default callbacks required for handling the client state
package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/auraspeak/client/internal/router"
	"github.com/auraspeak/client/pkg/command"
	"github.com/auraspeak/client/pkg/state"
	"github.com/auraspeak/protocol"
	"github.com/pion/dtls/v3"
	log "github.com/sirupsen/logrus"
)

// Gedanken:
// 	- ein isAlive wie im server, aber dies wird nachher über das protokoll gehändelt

// Client represents a DTLS-based UDP client that handles connections to the server and routes packets.
type Client struct {
	Host string
	Port int

	conn net.Conn

	// InsecureSkipVerify, when true, skips server certificate verification
	// (e.g. for self-signed certs in dev). Default false.
	InsecureSkipVerify bool

	ctx context.Context
	wg  sync.WaitGroup

	state.ClientState

	// Communication Channels
	// Client -> Server
	sendCh chan []byte
	// Server -> Client
	recvCh chan []byte
	// Sends errors from go Routines to main routine
	errCh chan error

	packetRouter *router.Router

	running bool

	// OutCommandCh sends internal commands to the web server (only used in debug builds)
	OutCommandCh chan command.InternalCommand
}

// NewClient creates a new UDP client with the given host and port.
func NewClient(Host string, Port int) *Client {
	ctx := context.Background()
	c := &Client{
		Host:         Host,
		Port:         Port,
		sendCh:       make(chan []byte),
		recvCh:       make(chan []byte),
		errCh:        make(chan error),
		ctx:          ctx,
		packetRouter: router.NewRouter(),
	}
	c.OnPacket(protocol.PacketTypeClientNeedsDisconnect, func(packet *protocol.Packet) error {
		log.WithField("caller", "client").Infof("Received ClientNeedsDisconnect from server, reason: %s", string(packet.Payload))
		c.Stop()
		return nil
	})
	return c
}

// OnPacket registers a new PacketHandler for a specific packet type
//
// Example:
//
//	client.OnPacket(protocol.PacketTypeDebugHello, func(packet *protocol.Packet) error {
//		fmt.Println("Received text packet:", string(packet))
//		return nil
//	})
func (c *Client) OnPacket(packetType protocol.PacketType, handler router.PacketHandler) {
	c.packetRouter.OnPacket(packetType, handler)
}

// Run starts the client and connects to the server.
func (c *Client) Run() error {
	connectionString := fmt.Sprintf("%s:%d", c.Host, c.Port)
	raddr, err := net.ResolveUDPAddr("udp4", connectionString)
	if err != nil {
		return err
	}
	c.conn, err = dtls.Dial("udp", raddr, &dtls.Config{InsecureSkipVerify: c.InsecureSkipVerify})
	if err != nil {
		return err
	}

	c.running = true
	c.SetRunningState(true)

	c.wg.Go(func() {
		c.recvLoop()
	})

	c.wg.Go(func() {
		c.sendLoop()
	})

	c.wg.Go(func() {
		c.handleErrors()
	})
	defer c.conn.Close()

	log.WithField("caller", "client").Info("Starting client")
	c.debugHello()
	c.wg.Wait()
	c.running = false
	c.SetRunningState(false)
	log.WithField("caller", "client").Info("Client Stopped")
	return nil
}

// Stop stops the client and closes the connection to the server.
func (c *Client) Stop() {
	c.running = false
	c.SetRunningState(false)
	if c.conn != nil {
		c.conn.Close()
	}
	c.wg.Wait()
	log.WithField("caller", "client").Info("Client Stopped")
}

// Send sends a packet to the server.
//
// Example:
//
//	client.Send([]byte("Hello, Server!"))
func (c *Client) Send(msg []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case <-c.ctx.Done():
		return errors.New("context cancelled")
	case <-ctx.Done():
		return errors.New("send timeout: sendLoop may not be running or is blocked")
	case c.sendCh <- msg:
		return nil
	}
}

// sendLoop sends packets to the Server
func (c *Client) sendLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-c.sendCh:
			if _, err := c.conn.Write(msg); err != nil {
				c.errCh <- err
				continue
			}
			c.SetRunningState(true)
		}
	}
}

// recvLoop receives packets from the Server
func (c *Client) recvLoop() {
	// TODO: change later the byte size
	buffer := make([]byte, 1024)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		n, err := c.conn.Read(buffer)
		if err != nil {
			c.errCh <- err
			continue
		}
		if n == 0 {
			continue
		}
		// TODO: same here change later when we exacly know the max msg length of a packet in the Protocol
		dst := make([]byte, n)
		copy(dst, buffer[:n])

		packet, err := protocol.Decode(dst)
		if err != nil {
			log.WithField("caller", "client").WithError(err).Error("Error decoding packet")
			continue
		}
		if err := c.packetRouter.HandlePacket(packet); err != nil {
			log.WithField("caller", "client").WithError(err).Error("Error handling packet")
			continue
		}
	}
}

func (c *Client) handleErrors() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case err := <-c.errCh:
			log.WithField("caller", "client").WithError(err).Error("Error in client")
			continue
		}
	}
}

// NewDebugClient creates a new debug client with the given host, port, and ID.
// This function is only available in debug builds.
func NewDebugClient(Host string, Port int, ID int) *Client {
	log.Error("NewDebugClient is not implemented in release build")
	return nil
}

func (c *Client) debugHello() {
	log.Error("debugHello is not implemented in release build")
}
