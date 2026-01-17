package main

import (
	"flag"
	"log"

	"github.com/Pujan-khunt/clipboard-sync/internal/client"
)

var (
	serverAddr = flag.String("server", "ws://localhost:8080/ws?room=default", "Signaling server WebSocket URL")
	password   = flag.String("password", "", "Password for E2E encryption (Required)")
	peerID     = flag.String("peerID", "", "Unique peer ID (auto-generated if empty)")
)

func main() {
	flag.Parse()

	app := client.NewApp(*serverAddr, *password, *peerID)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
