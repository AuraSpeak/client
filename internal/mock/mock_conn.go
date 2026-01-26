package mock

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// MockConn is a mock implementation of net.Conn for testing
type MockConn struct {
	readData    []byte
	readErr     error
	writeData   []byte
	writeErr    error
	closeErr    error
	localAddr   net.Addr
	remoteAddr  net.Addr
	closed      bool
	mu          sync.RWMutex
	readCalled  bool
	writeCalled bool
	closeCalled bool
}

// NewMockConn creates a new mock connection
func NewMockConn(localAddr, remoteAddr net.Addr) *MockConn {
	return &MockConn{
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
}

// Read implements net.Conn
func (m *MockConn) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCalled = true

	if m.closed {
		return 0, io.EOF
	}

	if m.readErr != nil {
		return 0, m.readErr
	}

	if len(m.readData) == 0 {
		// Block instead of EOF, so recvLoop continues
		// In real tests one should use SetReadData
		return 0, io.EOF
	}

	n = copy(b, m.readData)
	m.readData = m.readData[n:]
	return n, nil
}

// SetReadDataLoop sets data that will be returned repeatedly
func (m *MockConn) SetReadDataLoop(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// For loop tests: store data that is returned repeatedly
	m.readData = data
}

// Write implements net.Conn
func (m *MockConn) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalled = true

	if m.closed {
		return 0, errors.New("connection closed")
	}

	if m.writeErr != nil {
		return 0, m.writeErr
	}

	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

// Close implements net.Conn
func (m *MockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled = true
	m.closed = true
	return m.closeErr
}

// LocalAddr implements net.Conn
func (m *MockConn) LocalAddr() net.Addr {
	return m.localAddr
}

// RemoteAddr implements net.Conn
func (m *MockConn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

// SetDeadline implements net.Conn
func (m *MockConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements net.Conn
func (m *MockConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn
func (m *MockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// SetReadData sets the data to be returned by Read
func (m *MockConn) SetReadData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readData = data
}

// SetReadError sets the error to be returned by Read
func (m *MockConn) SetReadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readErr = err
}

// SetWriteError sets the error to be returned by Write
func (m *MockConn) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeErr = err
}

// GetWriteData returns the data written to the connection
func (m *MockConn) GetWriteData() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]byte, len(m.writeData))
	copy(result, m.writeData)
	return result
}

// IsClosed returns whether the connection is closed
func (m *MockConn) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

// WasReadCalled returns whether Read was called
func (m *MockConn) WasReadCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readCalled
}

// WasWriteCalled returns whether Write was called
func (m *MockConn) WasWriteCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writeCalled
}

// WasCloseCalled returns whether Close was called
func (m *MockConn) WasCloseCalled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closeCalled
}
