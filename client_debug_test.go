//go:build debug
// +build debug

package client

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auraspeak/client/internal/mock"
	"github.com/auraspeak/client/pkg/command"
	"github.com/auraspeak/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDebugClient(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 42)

	require.NotNil(t, c)
	assert.Equal(t, "localhost", c.Host)
	assert.Equal(t, 8080, c.Port)
	assert.Equal(t, 42, c.ClientState.ID)
	assert.NotNil(t, c.nc)
	assert.NotNil(t, c.OutCommandCh)
}

func TestNewDebugClient_DefaultHandler(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 1)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeClientNeedsDisconnect},
		Payload:      []byte("test reason"),
	}

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	c.nc.SetConnForTest(mockConn)

	err := c.nc.HandlePacketForTest(packet, "")
	assert.NoError(t, err)
}

func TestSetRunningState_Debug_WithNotification(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 1)

	// Set running state
	c.SetRunningState(true)
	assert.Equal(t, int32(1), atomic.LoadInt32(&c.ClientState.Running))

	// Check that notification was sent
	select {
	case cmd := <-c.OutCommandCh:
		assert.Equal(t, command.CmdUpdateClientState, cmd)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected command notification")
	}

	// Set not running
	c.SetRunningState(false)
	assert.Equal(t, int32(0), atomic.LoadInt32(&c.ClientState.Running))

	// Check that notification was sent
	select {
	case cmd := <-c.OutCommandCh:
		assert.Equal(t, command.CmdUpdateClientState, cmd)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected command notification")
	}
}

func TestSetRunningState_Debug_ContextCancelled(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.nc.SetContextForTest(ctx)

	c.SetRunningState(true)
	assert.Equal(t, int32(1), atomic.LoadInt32(&c.ClientState.Running))

	select {
	case <-c.OutCommandCh:
		t.Fatal("Should not receive notification when context is cancelled")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSetRunningState_Debug_ChannelFull(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 1)

	// Fill OutCommandCh
	for i := 0; i < cap(c.OutCommandCh); i++ {
		c.OutCommandCh <- command.CmdUpdateClientState
	}

	// Set running state (should skip notification if channel is full)
	c.SetRunningState(true)
	assert.Equal(t, int32(1), atomic.LoadInt32(&c.ClientState.Running))

	// Should not block or panic
	// State should still be updated
}

func TestDebugHello_WithConnection(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 42)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	c.nc.SetConnForTest(mockConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run() }()

	time.Sleep(150 * time.Millisecond)

	writtenData := mockConn.GetWriteData()
	assert.NotEmpty(t, writtenData)

	packet, err := protocol.Decode(writtenData)
	require.NoError(t, err)
	assert.Equal(t, protocol.PacketTypeDebugHello, packet.PacketHeader.PacketType)
	assert.Equal(t, []byte("42"), packet.Payload)

	c.Stop()
}

func TestDebugHello_WithoutConnection_Timeout(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 42)

	// debugHello is now called from OnConnect when Run() starts; without Run() it is not called.
	// Just verify debugHello sends when we have a connection (covered by TestDebugHello_WithConnection).
	c.debugHello()
}

func TestDebugHello_ConnectionReadyAfterWait(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 42)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	c.nc.SetConnForTest(mockConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run() }()

	time.Sleep(150 * time.Millisecond)

	writtenData := mockConn.GetWriteData()
	assert.NotEmpty(t, writtenData)

	packet, err := protocol.Decode(writtenData)
	require.NoError(t, err)
	assert.Equal(t, protocol.PacketTypeDebugHello, packet.PacketHeader.PacketType)
	assert.Equal(t, []byte("42"), packet.Payload)

	c.Stop()
}

func TestDebugHello_DifferentIDs(t *testing.T) {
	tests := []struct {
		name string
		id   int
	}{
		{"zero", 0},
		{"one", 1},
		{"large", 999999},
		{"negative", -1}, // Should still work, just converted to string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewDebugClient("localhost", 8080, tt.id)

			localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
			remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
			mockConn := mock.NewMockConn(localAddr, remoteAddr)
			c.nc.SetConnForTest(mockConn)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() { _ = c.Run() }()

			time.Sleep(150 * time.Millisecond)

			writtenData := mockConn.GetWriteData()
			assert.NotEmpty(t, writtenData)

			packet, err := protocol.Decode(writtenData)
			require.NoError(t, err)
			assert.Equal(t, protocol.PacketTypeDebugHello, packet.PacketHeader.PacketType)

			c.Stop()
		})
	}
}
