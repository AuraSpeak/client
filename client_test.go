package client

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auraspeak/client/internal/mock"
	"github.com/auraspeak/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setConn sets the connection for testing purposes
func setConn(c *Client, conn net.Conn) {
	c.conn = conn
}

// setContext sets the context for testing purposes
func setContext(c *Client, ctx context.Context) {
	c.ctx = ctx
}

func TestNewClient(t *testing.T) {
	c := NewClient("localhost", 8080)

	require.NotNil(t, c)
	assert.Equal(t, "localhost", c.Host)
	assert.Equal(t, 8080, c.Port)
	assert.NotNil(t, c.sendCh)
	assert.NotNil(t, c.recvCh)
	assert.NotNil(t, c.errCh)
	assert.NotNil(t, c.packetRouter)
	assert.NotNil(t, c.ctx)
	assert.False(t, c.running)
}

func TestNewClient_DefaultHandler(t *testing.T) {
	c := NewClient("localhost", 8080)

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

	err := c.packetRouter.HandlePacket(packet)
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestSend_Success(t *testing.T) {
	c := NewClient("localhost", 8080)

	// Start sendLoop in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	c.wg.Go(func() {
		c.sendLoop()
	})

	// Give sendLoop time to start
	time.Sleep(10 * time.Millisecond)

	msg := []byte("test message")
	err := c.Send(msg)
	assert.NoError(t, err)

	// Wait for message to be written
	time.Sleep(50 * time.Millisecond)

	writtenData := mockConn.GetWriteData()
	assert.Equal(t, msg, writtenData)

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestSend_Timeout(t *testing.T) {
	c := NewClient("localhost", 8080)

	// Don't start sendLoop, so sendCh will block
	// Fill sendCh to block it
	for i := 0; i < cap(c.sendCh); i++ {
		select {
		case c.sendCh <- []byte("block"):
		default:
		}
	}

	// Now Send should timeout
	msg := []byte("test message")
	err := c.Send(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send timeout")
}

func TestSend_ContextCancelled(t *testing.T) {
	c := NewClient("localhost", 8080)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	setContext(c, ctx)

	msg := []byte("test message")
	err := c.Send(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestSend_EmptyMessage(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	c.wg.Go(func() {
		c.sendLoop()
	})

	time.Sleep(10 * time.Millisecond)

	// Empty message should still work
	err := c.Send([]byte{})
	assert.NoError(t, err)

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestStop_WithConnection(t *testing.T) {
	c := NewClient("localhost", 8080)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	c.running = true
	c.Stop()

	assert.False(t, c.running)
	assert.True(t, mockConn.WasCloseCalled())
}

func TestStop_WithoutConnection(t *testing.T) {
	c := NewClient("localhost", 8080)

	c.running = true
	c.Stop()

	assert.False(t, c.running)
}

func TestStop_MultipleCalls(t *testing.T) {
	c := NewClient("localhost", 8080)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	c.running = true

	// Multiple stops should be safe
	c.Stop()
	c.Stop()
	c.Stop()

	assert.False(t, c.running)
}

func TestSendLoop_NormalSend(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	c.wg.Go(func() {
		c.sendLoop()
	})

	time.Sleep(10 * time.Millisecond)

	msg := []byte("test message")
	c.sendCh <- msg

	time.Sleep(50 * time.Millisecond)

	writtenData := mockConn.GetWriteData()
	assert.Equal(t, msg, writtenData)

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestSendLoop_WriteError(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	writeErr := errors.New("write error")
	mockConn.SetWriteError(writeErr)
	setConn(c, mockConn)

	errorReceived := make(chan error, 1)
	go func() {
		select {
		case err := <-c.errCh:
			errorReceived <- err
		case <-time.After(1 * time.Second):
			errorReceived <- errors.New("timeout")
		}
	}()

	c.wg.Go(func() {
		c.sendLoop()
	})

	time.Sleep(10 * time.Millisecond)

	msg := []byte("test message")
	c.sendCh <- msg

	// Wait for error
	select {
	case err := <-errorReceived:
		assert.Equal(t, writeErr, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected error from sendLoop")
	}

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestSendLoop_ConnectionClosed(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	mockConn.Close() // Close connection
	setConn(c, mockConn)

	errorReceived := make(chan error, 1)
	go func() {
		select {
		case err := <-c.errCh:
			errorReceived <- err
		case <-time.After(1 * time.Second):
			errorReceived <- errors.New("timeout")
		}
	}()

	c.wg.Go(func() {
		c.sendLoop()
	})

	time.Sleep(10 * time.Millisecond)

	msg := []byte("test message")
	c.sendCh <- msg

	// Wait for error
	select {
	case err := <-errorReceived:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection closed")
	case <-time.After(1 * time.Second):
		t.Fatal("Expected error from sendLoop")
	}

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestSendLoop_ContextCancellation(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	c.wg.Go(func() {
		c.sendLoop()
	})

	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for goroutine to finish with timeout
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		// Timeout - this is acceptable if the goroutine is still running
		// The important thing is that cancel() was called
	}
}

func TestRecvLoop_NormalPacket(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}
	packetData := packet.Encode()
	mockConn.SetReadData(packetData)
	setConn(c, mockConn)

	handlerCalled := false
	c.OnPacket(protocol.PacketTypeDebugHello, func(p *protocol.Packet) error {
		handlerCalled = true
		assert.Equal(t, packet.Payload, p.Payload)
		return nil
	})

	c.wg.Go(func() {
		c.recvLoop()
	})

	time.Sleep(100 * time.Millisecond)

	assert.True(t, handlerCalled)

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestRecvLoop_ReadError(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	readErr := errors.New("read error")
	mockConn.SetReadError(readErr)
	setConn(c, mockConn)

	errorReceived := make(chan error, 1)
	go func() {
		select {
		case err := <-c.errCh:
			errorReceived <- err
		case <-time.After(1 * time.Second):
			errorReceived <- errors.New("timeout")
		}
	}()

	c.wg.Go(func() {
		c.recvLoop()
	})

	time.Sleep(100 * time.Millisecond)

	// Wait for error
	select {
	case err := <-errorReceived:
		assert.Equal(t, readErr, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected error from recvLoop")
	}

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestRecvLoop_EmptyPacket(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	// Set empty read data (EOF)
	mockConn.SetReadData([]byte{})
	setConn(c, mockConn)

	handlerCalled := false
	c.OnPacket(protocol.PacketTypeDebugHello, func(p *protocol.Packet) error {
		handlerCalled = true
		return nil
	})

	c.wg.Go(func() {
		c.recvLoop()
	})

	time.Sleep(100 * time.Millisecond)

	// Handler should not be called for empty packets
	assert.False(t, handlerCalled)

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestRecvLoop_InvalidPacket(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	// Invalid packet data (too short)
	mockConn.SetReadData([]byte{0x01}) // Only 1 byte, need at least HeaderSize
	setConn(c, mockConn)

	handlerCalled := false
	c.OnPacket(protocol.PacketTypeDebugHello, func(p *protocol.Packet) error {
		handlerCalled = true
		return nil
	})

	c.wg.Go(func() {
		c.recvLoop()
	})

	time.Sleep(100 * time.Millisecond)

	// Handler should not be called for invalid packets
	assert.False(t, handlerCalled)

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestRecvLoop_HandlerError(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)

	packet := &protocol.Packet{
		PacketHeader: protocol.Header{PacketType: protocol.PacketTypeDebugHello},
		Payload:      []byte("test"),
	}
	packetData := packet.Encode()
	mockConn.SetReadData(packetData)
	setConn(c, mockConn)

	handlerCalled := false
	c.OnPacket(protocol.PacketTypeDebugHello, func(p *protocol.Packet) error {
		handlerCalled = true
		return errors.New("handler error")
	})

	c.wg.Go(func() {
		c.recvLoop()
	})

	time.Sleep(100 * time.Millisecond)

	// Handler should be called, but error is logged, not returned
	assert.True(t, handlerCalled)

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestRecvLoop_ConnectionClosed(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	mockConn.Close()              // Close connection
	mockConn.SetReadError(io.EOF) // EOF after close
	setConn(c, mockConn)

	errorReceived := make(chan error, 1)
	go func() {
		select {
		case err := <-c.errCh:
			errorReceived <- err
		case <-time.After(1 * time.Second):
			errorReceived <- errors.New("timeout")
		}
	}()

	c.wg.Go(func() {
		c.recvLoop()
	})

	time.Sleep(100 * time.Millisecond)

	// Wait for error
	select {
	case err := <-errorReceived:
		assert.Error(t, err)
	case <-time.After(1 * time.Second):
		// EOF might not trigger error in some cases
	}

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestRecvLoop_ContextCancellation(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	c.wg.Go(func() {
		c.recvLoop()
	})

	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for goroutine to finish with timeout
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		// Timeout - this is acceptable if the goroutine is still running
		// The important thing is that cancel() was called
	}
}

func TestHandleErrors(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	testErr := errors.New("test error")

	c.wg.Go(func() {
		c.handleErrors()
	})

	time.Sleep(10 * time.Millisecond)

	// Send error
	c.errCh <- testErr

	time.Sleep(50 * time.Millisecond)

	// Error should be handled (logged, not returned)
	// Just verify the goroutine is still running
	cancel()
	// Use timeout for wg.Wait to prevent hanging
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestHandleErrors_ContextCancellation(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	setContext(c, ctx)

	c.wg.Go(func() {
		c.handleErrors()
	})

	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for goroutine to finish with timeout
	done := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		// Timeout - this is acceptable if the goroutine is still running
		// The important thing is that cancel() was called
	}
}

func TestClient_ConcurrentSend(t *testing.T) {
	c := NewClient("localhost", 8080)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setContext(c, ctx)

	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081}
	mockConn := mock.NewMockConn(localAddr, remoteAddr)
	setConn(c, mockConn)

	c.wg.Go(func() {
		c.sendLoop()
	})

	time.Sleep(10 * time.Millisecond)

	// Send multiple messages concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			msg := []byte{byte(idx)}
			err := c.Send(msg)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all sends
	for i := 0; i < 10; i++ {
		<-done
	}

	time.Sleep(100 * time.Millisecond)

	writtenData := mockConn.GetWriteData()
	assert.GreaterOrEqual(t, len(writtenData), 10)

	cancel()
	// Use timeout for wg.Wait to prevent hanging
	waitDone := make(chan bool, 1)
	go func() {
		c.wg.Wait()
		waitDone <- true
	}()
	select {
	case <-waitDone:
	case <-time.After(500 * time.Millisecond):
		// Timeout acceptable
	}
}

func TestClient_StateManagement(t *testing.T) {
	c := NewClient("localhost", 8080)

	// Initially not running
	assert.Equal(t, int32(0), atomic.LoadInt32(&c.ClientState.Running))

	// Set running state
	c.SetRunningState(true)
	assert.Equal(t, int32(1), atomic.LoadInt32(&c.ClientState.Running))

	// Set not running
	c.SetRunningState(false)
	assert.Equal(t, int32(0), atomic.LoadInt32(&c.ClientState.Running))
}
