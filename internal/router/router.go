// Package router provides packet routing functionality for the client.
package router

import (
	"errors"
	"sync"

	"github.com/auraspeak/protocol"
)

// PacketHandler is a function type that handles incoming packets.
type PacketHandler func(packet *protocol.Packet) error

// Router routes incoming packets to their registered handlers based on packet type.
type Router struct {
	handlers sync.Map // packetType -> PacketHandler
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	return &Router{
		handlers: sync.Map{},
	}
}

// OnPacket registers a new PacketHandler for a specific packet type
//
// Example:
//
//	router.OnPacket(protocol.PacketTypeDebugHello, func(packet *protocol.Packet) error {
//		fmt.Println("Received text packet:", string(packet))
//		return nil
//	})
func (r *Router) OnPacket(packetType protocol.PacketType, handler PacketHandler) {
	r.handlers.Store(packetType, handler)
}

// HandlePacket routes a packet to the appropriate handler.
//
// Example:
//
//	err := router.HandlePacket(packet)
//	if err != nil {
//		fmt.Println("Error routing packet:", err)
//	}
func (r *Router) HandlePacket(packet *protocol.Packet) error {
	// Check if the packet type is valid
	if !protocol.IsValidPacketType(packet.PacketHeader.PacketType) {
		return errors.New("invalid packet type")
	}
	// Load the handler for the packet type
	handler, ok := r.handlers.Load(packet.PacketHeader.PacketType)
	if !ok {
		return errors.New("no handler found for packet type")
	}
	// Cast the handler to the PacketHandler type
	handlerFunc := handler.(PacketHandler)
	// Call the handler function
	return handlerFunc(packet)
}
