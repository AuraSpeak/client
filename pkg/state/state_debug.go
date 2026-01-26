//go:build debug
// +build debug

package state

// ClientState holds the current state of the client (debug build, includes ID for web server).
type ClientState struct {
	ID      int   `json:"id"`
	Running int32 `json:"running"`
}
