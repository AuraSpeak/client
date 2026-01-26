package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/auraspeak/client"
	"github.com/auraspeak/protocol"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Standardwerte
	host := "localhost"
	port := 8080

	// Client erstellen
	c := client.NewClient(host, port)

	// Message Handler registrieren
	c.OnPacket(protocol.PacketTypeDebugAny, func(packet *protocol.Packet) error {
		fmt.Printf("Empfangen: %s\n", string(packet.Payload))
		return nil
	})

	// Client in Goroutine starten
	errCh := make(chan error, 1)
	go func() {
		if err := c.Run(); err != nil {
			errCh <- err
		}
	}()

	// Signal Handler für graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Eingabe-Loop in separater Goroutine
	inputCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("Client gestartet. Tippe Nachrichten ein (oder 'quit' zum Beenden):")
		for scanner.Scan() {
			inputCh <- scanner.Text()
		}
	}()

	// Haupt-Loop
	for {
		select {
		case <-sigCh:
			fmt.Println("\nBeende Client...")
			return
		case err := <-errCh:
			log.WithError(err).Error("Client Fehler")
			return
		case text := <-inputCh:
			if text == "quit" {
				fmt.Println("Beende Client...")
				return
			}
			if err := c.Send([]byte(text)); err != nil {
				log.WithError(err).Error("Fehler beim Senden")
			}
		}
	}
}
