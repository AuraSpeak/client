package client

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auraspeak/client/internal/mock"
	"github.com/auraspeak/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	c := NewClient("localhost", 8080)

	require.NotNil(t, c)
	assert.Equal(t, "localhost", c.Host)
	assert.Equal(t, 8080, c.Port)
	assert.NotNil(t, c.nc)
	assert.NotNil(t, c.OutCommandCh)
}

func TestNewClient_DefaultHandler(t *testing.T) {
	c := NewClient("localhost", 8080)

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

func TestOnPacket(t *testing.T) {
	c := NewClient("localhost", 8080)
	handlerCalled := false

	handler := func(packet *protocol.Packet) error {
		handlerCalled = true
		return nil
	}

	c.OnPacket(protocol.PacketTypeDebugHello, handler)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}

	err := c.nc.HandlePacketForTest(packet, "")
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestSend_Success(t *testing.T) {
	c := NewClient("localhost", 8080)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	c.nc.SetConnForTest(mockConn)

	go func() {
		_ = c.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	msg := []byte("test message")
	err := c.Send(msg)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	writtenData := mockConn.GetWriteData()
	assert.Equal(t, msg, writtenData)

	c.Stop()
}

func TestSend_Timeout(t *testing.T) {
	c := NewClient("localhost", 8080)

	msg := []byte("test message")
	err := c.Send(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send timeout")
}

func TestSend_ContextCancelled(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.nc.SetContextForTest(ctx)

	msg := []byte("test message")
	err := c.Send(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestSend_EmptyMessage(t *testing.T) {
	c := NewClient("localhost", 8080)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	c.nc.SetConnForTest(mockConn)

	go func() {
		_ = c.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	err := c.Send([]byte{})
	assert.NoError(t, err)

	c.Stop()
}

func TestStop_WithConnection(t *testing.T) {
	c := NewClient("localhost", 8080)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	c.nc.SetConnForTest(mockConn)

	go func() {
		_ = c.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	c.Stop()

	assert.True(t, mockConn.WasCloseCalled())
}

func TestStop_WithoutConnection(t *testing.T) {
	c := NewClient("localhost", 8080)

	c.Stop()
}

func TestStop_MultipleCalls(t *testing.T) {
	c := NewClient("localhost", 8080)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	c.nc.SetConnForTest(mockConn)

	go func() {
		_ = c.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	c.Stop()
	c.Stop()
	c.Stop()
}

func TestClient_StateManagement(t *testing.T) {
	c := NewClient("localhost", 8080)

	assert.Equal(t, int32(0), atomic.LoadInt32(&c.Running))

	c.SetRunningState(true)
	assert.Equal(t, int32(1), atomic.LoadInt32(&c.Running))

	c.SetRunningState(false)
	assert.Equal(t, int32(0), atomic.LoadInt32(&c.Running))
}

func TestRun_ReceivesPacket(t *testing.T) {
	c := NewClient("localhost", 8080)

	handlerCalled := make(chan bool, 1)
	c.OnPacket(protocol.PacketTypeDebugHello, func(p *protocol.Packet) error {
		handlerCalled <- true
		assert.Equal(t, []byte("test"), p.Payload)
		return nil
	})

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}
	mockConn.SetReadData(packet.Encode())
	c.nc.SetConnForTest(mockConn)

	go func() {
		_ = c.Run()
	}()

	select {
	case <-handlerCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler was not called")
	}

	c.Stop()
}
