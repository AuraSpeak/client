//go:build !debug
// +build !debug

package client

import "sync/atomic"

// SetRunningState updates the running state of the client atomically (release builds only, no web server notification).
func (c *Client) SetRunningState(running bool) {
	var v int32
	if running {
		v = 1
	}
	atomic.StoreInt32(&c.Running, v)
	// OutCommandCh is nil in non-debug builds, so no notification is sent
}
