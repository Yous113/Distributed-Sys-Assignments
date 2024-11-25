package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	auction "cookAssignment5/node"
)

var (
	ipFlag   = flag.String("ip", "127.0.0.1", "IP address of this node")
	portFlag = flag.Int("port", 5000, "Port number of this node")
)

func main() {
	// Parse command-line flags
	flag.Parse()

	// Get IP and port for this node
	ip := *ipFlag
	port := *portFlag

	// Initialize the auction node
	node := auction.InitializeNode(ip, port)

	// Read peer nodes from the configuration file
	peerAddresses := readPeerNodes("nodes.txt")

	// Launch the auction system
	fmt.Printf("Node starting with IP: %s, Port: %d\n", ip, port)
	node.Launch(peerAddresses)

	// Keep the node running
	select {}
}

func readPeerNodes(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open nodes file: %v", err)
	}
	defer file.Close()

	var peers []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		// Ignore empty lines or comments
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
			peers = append(peers, line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading nodes file: %v", err)
	}

	return peers
}
