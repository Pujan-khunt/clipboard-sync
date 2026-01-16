package main

import (
	"flag"
	"log"

	"github.com/Pujan-khunt/clipboard-sync/internal/client"
)

var (
	serverAddr = flag.String("serverAddr", "ws://localhost:8080/ws?room=default", "Server WebSocket URL")
	password   = flag.String("password", "", "Password for encryption (Required)")
)

func main() {
	flag.Parse()

	app := client.NewApp(*serverAddr, *password)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
