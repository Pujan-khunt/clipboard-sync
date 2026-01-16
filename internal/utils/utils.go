// Package utils contains utility functions.
package utils

import (
	"fmt"
	"log"
	"net"
)

// PrintLocalIPs prints the interfaces on which the server is going to be listening on.
//
// Parameters:
//   - port: includes the ':' before the actual port number
func PrintLocalIPs(port string) {
	fmt.Println(">> Available Network Addresses:")

	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Error getting interfaces: %v", err)
		return
	}

	for _, iface := range interfaces {
		// Skip interfaces which are down or loopback interfaces (eg. localhost)
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Get addresses for these interfaces
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Print only IPv4 addresses
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}

			fmt.Printf("    - ws://%s%s/ws\n", ip.String(), port)
		}

	}
	fmt.Printf("    - ws://localhost%s/ws (Local only)\n", port)
	fmt.Println("----------------------------------------------")
}
