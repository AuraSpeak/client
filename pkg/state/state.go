//go:build !debug
// +build !debug

// Package state defines the client state structure.
package state

// ClientState holds the current state of the client (release build).
type ClientState struct {
	Running int32
}
