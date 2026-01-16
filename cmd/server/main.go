package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Pujan-khunt/clipboard-sync/internal/utils"
	"github.com/Pujan-khunt/clipboard-sync/internal/wsserver"
)

var port = flag.String("port", ":8080", "Port to listen on")

func main() {
	flag.Parse()

	hub := wsserver.NewHub()

	http.HandleFunc("/ws", hub.HandleConnections)

	utils.PrintLocalIPs(*port)
	if err := http.ListenAndServe(*port, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
