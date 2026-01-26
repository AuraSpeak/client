// Package command defines internal client commands.
package command

// InternalCommand represents a command type for internal client operations.
type InternalCommand int

// Internal commands for client operations.
const (
	// CmdUpdateClientState signals that the client state should be updated.
	CmdUpdateClientState InternalCommand = iota
)
