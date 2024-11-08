package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	pb "assignment4/proto"

	"google.golang.org/grpc"
)

// Node represents a node in the token ring
type Node struct {
	pb.UnimplementedTokenRingServer
	nodeID         string
	address        string
	successorAddr  string
	client         pb.TokenRingClient
	tokenAvailable bool
	mutex          sync.Mutex
	requestQueue   []string
}

// NewNode initializes a new Node
func NewNode(nodeID, address, successorAddr string) *Node {
	return &Node{
		nodeID:         nodeID,
		address:        address,
		successorAddr:  successorAddr,
		tokenAvailable: false,
		requestQueue:   []string{},
	}
}

// RequestToken handles incoming token requests
func (n *Node) RequestToken(ctx context.Context, req *pb.TokenRequest) (*pb.Acknowledgement, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	log.Printf("Received token request from %s", req.NodeId)
	n.requestQueue = append(n.requestQueue, req.NodeId)
	return &pb.Acknowledgement{Success: true}, nil
}

// PassToken handles receiving the token
func (n *Node) PassToken(ctx context.Context, token *pb.Token) (*pb.Acknowledgement, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	log.Printf("Node %s received Token", n.nodeID)
	n.tokenAvailable = true
	go n.enterCriticalSection()
	return &pb.Acknowledgement{Success: true}, nil
}

// enterCriticalSection attempts to enter the critical section if the token is available
func (n *Node) enterCriticalSection() {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.tokenAvailable && len(n.requestQueue) > 0 && n.requestQueue[0] == n.nodeID {
		n.tokenAvailable = false
		n.requestQueue = n.requestQueue[1:]

		log.Printf("Node %s entering Critical Section", n.nodeID)
		time.Sleep(2 * time.Second) // Simulate some work
		log.Printf("Node %s exiting Critical Section", n.nodeID)

		n.passToken()
	}
}

// passToken sends the token to the successor node
func (n *Node) passToken() {
	log.Printf("Node %s passing token to %s", n.nodeID, n.successorAddr)
	conn, err := grpc.Dial(n.successorAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to successor %s: %v", n.successorAddr, err)
	}
	defer conn.Close()

	client := pb.NewTokenRingClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = client.PassToken(ctx, &pb.Token{})
	if err != nil {
		log.Fatalf("Failed to pass token to %s: %v", n.successorAddr, err)
	}
}

// requestCriticalSection requests access to the Critical Section
func (n *Node) requestCriticalSection() {
	n.mutex.Lock()
	n.requestQueue = append(n.requestQueue, n.nodeID)
	n.mutex.Unlock()

	log.Printf("Node %s requesting token", n.nodeID)

	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.tokenAvailable && n.requestQueue[0] == n.nodeID {
		n.tokenAvailable = false
		n.requestQueue = n.requestQueue[1:]

		log.Printf("Node %s entering Critical Section", n.nodeID)
		time.Sleep(2 * time.Second) // Simulate some work
		log.Printf("Node %s exiting Critical Section", n.nodeID)

		n.passToken()
	}
}

func main() {
	logfile, err := os.OpenFile("tokenring.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logfile.Close()

	multiWriter := io.MultiWriter(logfile, os.Stdout)
	log.SetOutput(multiWriter)

	nodeID := flag.String("id", "", "Unique ID of the node")
	address := flag.String("addr", "", "Address of the node (e.g., localhost:5001)")
	successor := flag.String("succ", "", "Address of the successor node")
	flag.Parse()

	if *nodeID == "" || *address == "" || *successor == "" {
		log.Println("Usage: go run node.go -id=<node_id> -addr=<node_address> -succ=<successor_address>")
		os.Exit(1)
	}

	node := NewNode(*nodeID, *address, *successor)

	lis, err := net.Listen("tcp", node.address)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", node.address, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterTokenRingServer(grpcServer, node)

	if node.nodeID == "1" {
		node.tokenAvailable = true
		go node.enterCriticalSection()
	}

	go func() {
		log.Printf("Node %s listening at %s", node.nodeID, node.address)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// fictional scenario where nodes request the critical section
	for {
		time.Sleep(time.Duration(7+nodeIDHash(node.nodeID)) * time.Second)
		node.requestCriticalSection()
	}
}

func nodeIDHash(id string) int {
	var sum int
	for _, c := range id {
		sum += int(c)
	}
	return sum % 5
}
