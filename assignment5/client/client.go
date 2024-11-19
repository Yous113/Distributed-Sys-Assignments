package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"assignment5/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client represents a participant in the auction system.
type Client struct {
	id           int32
	currentBid   int32
	serverClient proto.AuctionServiceClient
	mu           sync.Mutex
}

// Command-line flags
var (
	clientIDFlag = flag.Int("id", 0, "Unique identifier for the client")
	nodesFile    = flag.String("nodes", "../Servernodes.txt", "Path to the nodes configuration file")
)

// main initializes the client and starts the interaction loop.
func main() {
	flag.Parse()

	if *clientIDFlag == 0 {
		log.Fatalf("Client ID must be provided using the -id flag.")
	}

	client := &Client{
		id:         int32(*clientIDFlag),
		currentBid: 0,
	}

	err := client.connectToServer(*nodesFile)
	if err != nil {
		log.Fatalf("Failed to connect to any server: %v", err)
	}

	client.run()
}

// connectToServer reads the nodes configuration and attempts to connect to a random server.
func (c *Client) connectToServer(nodesPath string) error {
	nodes, err := readNodesFromFile(nodesPath)
	if err != nil {
		return fmt.Errorf("error reading nodes file: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in the configuration file")
	}

	rand.Seed(time.Now().UnixNano())
	shuffledNodes := shuffleNodes(nodes)

	for _, node := range shuffledNodes {
		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", node.IP, node.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Unable to connect to node %s:%d. Error: %v", node.IP, node.Port, err)
			continue
		}
		c.serverClient = proto.NewAuctionServiceClient(conn)
		log.Printf("Successfully connected to server at %s:%d", node.IP, node.Port)
		return nil
	}

	return fmt.Errorf("could not connect to any server")
}

// readNodesFromFile parses the nodes configuration file.
func readNodesFromFile(filePath string) ([]NodeInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var nodes []NodeInfo
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			log.Printf("Invalid node entry: %s. Skipping.", line)
			continue
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Printf("Invalid port number in entry: %s. Skipping.", line)
			continue
		}
		nodes = append(nodes, NodeInfo{IP: parts[0], Port: port})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nodes, nil
}

// NodeInfo holds the IP and port of a server node.
type NodeInfo struct {
	IP   string
	Port int
}

// shuffleNodes randomizes the order of nodes to distribute connection attempts.
func shuffleNodes(nodes []NodeInfo) []NodeInfo {
	shuffled := make([]NodeInfo, len(nodes))
	copy(shuffled, nodes)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled
}

// MakeBid attempts to place a new bid if it's higher than the current bid.
func (c *Client) MakeBid(bidAmount int32) {
	c.mu.Lock()
	if bidAmount <= c.currentBid {
		log.Printf("Your bid of %d is not higher than the current bid of %d.", bidAmount, c.currentBid)
		c.mu.Unlock()
		return
	}
	c.currentBid = bidAmount
	c.mu.Unlock()

	req := &proto.BidRequest{
		Id:          c.id,
		Amount:      c.currentBid,
		Coordinator: false,
	}

	for {
		_, err := c.serverClient.Bid(context.Background(), req)
		if err != nil {
			log.Printf("Bid failed: %v. Attempting to reconnect...", err)
			err = c.connectToServer(*nodesFile)
			if err != nil {
				log.Printf("Reconnection failed: %v. Retrying in 5 seconds...", err)
				time.Sleep(5 * time.Second)
				continue
			}
			continue
		}
		log.Printf("Successfully placed a bid of %d.", c.currentBid)
		break
	}
}

// GetResult retrieves the current state or final result of the auction.
func (c *Client) GetResult() {
	req := &proto.RequestResult{}

	for {
		resp, err := c.serverClient.GetResult(context.Background(), req)
		if err != nil {
			log.Printf("Failed to get result: %v. Attempting to reconnect...", err)
			err = c.connectToServer(*nodesFile)
			if err != nil {
				log.Printf("Reconnection failed: %v. Retrying in 5 seconds...", err)
				time.Sleep(5 * time.Second)
				continue
			}
			continue
		}
		fmt.Println("Auction Result:", resp.Result)
		break
	}
}

// run initiates the interactive loop for the client to place bids or request results.
func (c *Client) run() {

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Enter command: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		switch strings.ToLower(input) {
		case "result":
			go c.GetResult()
		case "exit":
			fmt.Println("Exiting the auction client.")
			return
		default:
			bid, err := strconv.Atoi(input)
			if err != nil || bid <= 0 {
				fmt.Println("Invalid input. Please enter a positive integer for bids or 'result'/'exit'.")
				continue
			}
			go c.MakeBid(int32(bid))
		}
	}
}
