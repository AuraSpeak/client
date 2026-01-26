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
	assert.NotNil(t, c.sendCh)
	assert.NotNil(t, c.recvCh)
	assert.NotNil(t, c.errCh)
	assert.NotNil(t, c.packetRouter)
	assert.NotNil(t, c.ctx)
	assert.NotNil(t, c.OutCommandCh)
	assert.False(t, c.running)
}

func TestNewDebugClient_DefaultHandler(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 1)

	// Check that ClientNeedsDisconnect handler is registered
	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeClientNeedsDisconnect},
		Payload:      []byte("test reason"),
	}

	// Set a mock connection so Stop() doesn't panic
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	// Handler should call Stop()
	err := c.packetRouter.HandlePacket(packet)
	assert.NoError(t, err)
	assert.False(t, c.running)
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

	// Cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	setContext(c, ctx)

	// Set running state
	c.SetRunningState(true)
	assert.Equal(t, int32(1), atomic.LoadInt32(&c.ClientState.Running))

	// Notification should not be sent (context cancelled)
	select {
	case <-c.OutCommandCh:
		t.Fatal("Should not receive notification when context is cancelled")
	case <-time.After(100 * time.Millisecond):
		// Expected - no notification
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	// Start sendLoop
	c.wg.Go(func() {
		c.sendLoop()
	})

	time.Sleep(10 * time.Millisecond)

	// Call debugHello
	c.debugHello()

	time.Sleep(100 * time.Millisecond)

	// Check that debug hello packet was sent
	writtenData := mockConn.GetWriteData()
	assert.NotEmpty(t, writtenData)

	// Decode and verify packet
	packet, err := protocol.Decode(writtenData)
	require.NoError(t, err)
	assert.Equal(t, protocol.PacketTypeDebugHello, packet.PacketHeader.PacketType)
	assert.Equal(t, []byte("42"), packet.Payload)

	cancel()
	c.wg.Wait()
}

func TestDebugHello_WithoutConnection_Timeout(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 42)

	// Don't set connection, so debugHello should timeout
	// This test verifies the timeout behavior
	start := time.Now()
	c.debugHello()
	duration := time.Since(start)

	// Should timeout after ~5 seconds
	assert.GreaterOrEqual(t, duration, 4*time.Second)
	assert.Less(t, duration, 6*time.Second)
}

func TestDebugHello_ConnectionReadyAfterWait(t *testing.T) {
	c := NewDebugClient("localhost", 8080, 42)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)

	// Start sendLoop
	c.wg.Go(func() {
		c.sendLoop()
	})

	time.Sleep(10 * time.Millisecond)

	// Set connection after a short delay (simulating connection establishment)
	go func() {
		time.Sleep(50 * time.Millisecond)
		setConn(c, mockConn)
	}()

	// Call debugHello (should wait for connection)
	c.debugHello()

	time.Sleep(100 * time.Millisecond)

	// Check that debug hello packet was sent
	writtenData := mockConn.GetWriteData()
	assert.NotEmpty(t, writtenData)

	// Decode and verify packet
	packet, err := protocol.Decode(writtenData)
	require.NoError(t, err)
	assert.Equal(t, protocol.PacketTypeDebugHello, packet.PacketHeader.PacketType)
	assert.Equal(t, []byte("42"), packet.Payload)

	cancel()
	c.wg.Wait()
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

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			setContext(c, ctx)

			localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
			remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
			mockConn := mock.NewMockConn(localAddr, remoteAddr)
			setConn(c, mockConn)

			// Start sendLoop
			c.wg.Go(func() {
				c.sendLoop()
			})

			time.Sleep(10 * time.Millisecond)

			c.debugHello()

			time.Sleep(100 * time.Millisecond)

			writtenData := mockConn.GetWriteData()
			assert.NotEmpty(t, writtenData)

			packet, err := protocol.Decode(writtenData)
			require.NoError(t, err)
			assert.Equal(t, protocol.PacketTypeDebugHello, packet.PacketHeader.PacketType)

			cancel()
			c.wg.Wait()
		})
	}
}
