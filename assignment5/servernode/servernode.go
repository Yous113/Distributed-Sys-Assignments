package servernode

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"assignment5/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ServerNode represents a single node in the distributed auction system.
type ServerNode struct {
	proto.UnimplementedAuctionServiceServer
	addressIp     string
	port          int
	uniqueID      int32
	leaderID      int32
	nodeIDToIndex map[int32]int
	neighbors     []proto.AuctionServiceClient
	bidsHistory   map[int32]int32
	actionOngoing bool
	lock          sync.Mutex
}

// NewServerNode initializes a new ServerNode with the given IP and port.
func NewServerNode(ip string, port int) *ServerNode {
	return &ServerNode{
		addressIp:     ip,
		port:          port,
		uniqueID:      int32(port), // Assuming port uniquely identifies the node
		nodeIDToIndex: make(map[int32]int),
		bidsHistory:   make(map[int32]int32),
		actionOngoing: true,
	}
}

// Start initializes the gRPC server and connects to other nodes.
func (n *ServerNode) Start(nodeAddresses []string) {
	go n.startServer()

	// Allow some time for the server to start
	time.Sleep(2 * time.Second)

	// Connect to other nodes
	for _, addr := range nodeAddresses {
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			log.Printf("Invalid node address: %s", addr)
			continue
		}
		ip := parts[0]
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Printf("Invalid port in address %s: %v", addr, err)
			continue
		}

		if port == n.port {
			// Current node; skip
			continue
		}

		client, err := n.connectToNode(ip, port)
		if err != nil {
			log.Printf("Failed to connect to node %s:%d: %v", ip, port, err)
			continue
		}
		n.nodeIDToIndex[int32(port)] = len(n.neighbors)
		n.neighbors = append(n.neighbors, client)
	}

	// Assume the node with the smallest ID is the leader initially
	n.determineInitialLeader(nodeAddresses)

	// Start auction timer
	go n.runAuction()

	// Simulate potential crash
	go n.simulateCrash()

	select {} // Block forever
}

// startServer starts the gRPC server to listen for incoming requests.
func (n *ServerNode) startServer() {
	address := fmt.Sprintf(":%d", n.port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", address, err)
	}
	grpcServer := grpc.NewServer()
	proto.RegisterAuctionServiceServer(grpcServer, n)
	log.Printf("ServerNode %d started and listening on %s", n.uniqueID, address)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}

// connectToNode establishes a gRPC connection to another node.
func (n *ServerNode) connectToNode(ip string, port int) (proto.AuctionServiceClient, error) {
	address := fmt.Sprintf("%s:%d", ip, port)
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return proto.NewAuctionServiceClient(conn), nil
}

// determineInitialLeader selects the leader based on the smallest node ID.
func (n *ServerNode) determineInitialLeader(nodeAddresses []string) {
	minID := n.uniqueID
	for nodeID := range n.nodeIDToIndex {
		if nodeID < minID {
			minID = nodeID
		}
	}
	n.leaderID = minID
	if n.leaderID == n.uniqueID {
		log.Printf("ServerNode %d is the initial leader", n.uniqueID)
	}
}

// runAuction manages the auction lifecycle.
func (n *ServerNode) runAuction() {
	log.Printf("Auction started by ServerNode %d", n.uniqueID)
	auctionDuration := 100 * time.Second // Adjust as needed
	time.Sleep(auctionDuration)
	n.lock.Lock()
	n.actionOngoing = false
	n.lock.Unlock()
	log.Printf("Auction finished by ServerNode %d. Winner: %s", n.uniqueID, n.getWinner())
}

// getWinner determines the highest bid and returns the winner's information.
func (n *ServerNode) getWinner() string {
	n.lock.Lock()
	defer n.lock.Unlock()

	if len(n.bidsHistory) == 0 {
		return "No bids placed."
	}

	var highestBidder int32
	var highestBid int32
	for bidder, bid := range n.bidsHistory {
		if bid > highestBid {
			highestBid = bid
			highestBidder = bidder
		}
	}
	return fmt.Sprintf("Bidder %d with a bid of %d", highestBidder, highestBid)
}

// Bid handles incoming bid requests.
func (n *ServerNode) Bid(ctx context.Context, req *proto.BidRequest) (*proto.Ack, error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if !n.actionOngoing {
		log.Printf("Bid rejected: Auction has ended.")
		return &proto.Ack{}, nil
	}

	if req.Amount <= n.getCurrentHighestBid() {
		log.Printf("Bid rejected: Bid amount %d is not higher than current highest bid %d.", req.Amount, n.getCurrentHighestBid())
		return &proto.Ack{}, nil
	}

	if n.leaderID == n.uniqueID {
		// Leader processes the bid
		n.bidsHistory[req.Id] = req.Amount
		log.Printf("Leader ServerNode %d received bid: Bidder %d with amount %d", n.uniqueID, req.Id, req.Amount)
		// Replicate the bid to followers
		for _, neighbor := range n.neighbors {
			go func(client proto.AuctionServiceClient) {
				replicaReq := &proto.BidRequest{
					Id:          req.Id,
					Amount:      req.Amount,
					Coordinator: true, // Indicate that this bid is replicated from leader
				}
				_, err := client.Bid(context.Background(), replicaReq)
				if err != nil {
					log.Printf("Failed to replicate bid to a follower: %v", err)
				}
			}(neighbor)
		}
		return &proto.Ack{}, nil
	}

	// Followers forward the bid to the leader
	leaderClient, err := n.getLeaderClient()
	if err != nil {
		log.Printf("Leader not available. Initiating election.")
		n.initiateElection()
		return &proto.Ack{}, nil
	}

	_, err = leaderClient.Bid(ctx, req)
	if err != nil {
		log.Printf("Failed to forward bid to leader: %v", err)
		n.initiateElection()
		return &proto.Ack{}, nil
	}
	return &proto.Ack{}, nil
}

// GetResult handles requests to get the auction result.
func (n *ServerNode) GetResult(ctx context.Context, req *proto.RequestResult) (*proto.Result, error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.leaderID != n.uniqueID {
		// Forward the request to the leader
		leaderClient, err := n.getLeaderClient()
		if err != nil {
			log.Printf("Leader not available. Cannot get result.")
			return &proto.Result{Result: "Leader not available."}, nil
		}
		return leaderClient.GetResult(ctx, req)
	}

	// Leader responds with the result
	if n.actionOngoing {
		currentWinner := n.getCurrentHighestBidder()
		return &proto.Result{
			Result: fmt.Sprintf("Auction in progress. Current highest bid: %d by Bidder %d",
				n.getCurrentHighestBid(), currentWinner),
		}, nil
	}

	winner := n.getWinner()
	return &proto.Result{Result: winner}, nil
}

// ExecuteElection handles incoming election requests.
func (n *ServerNode) ExecuteElection(ctx context.Context, req *proto.Status) (*proto.Ack, error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if req.ServerId > n.leaderID {
		n.leaderID = req.ServerId
		log.Printf("ServerNode %d acknowledges new leader: ServerNode %d", n.uniqueID, n.leaderID)
	}
	return &proto.Ack{}, nil
}

// initiateElection starts a leader election process.
func (n *ServerNode) initiateElection() {
	log.Printf("ServerNode %d is initiating an election.", n.uniqueID)
	// Simple election: select the node with the highest ID as the leader
	newLeaderID := n.uniqueID
	for nodeID := range n.nodeIDToIndex {
		if nodeID > newLeaderID {
			newLeaderID = nodeID
		}
	}
	if newLeaderID == n.uniqueID {
		log.Printf("ServerNode %d elected as the new leader.", n.leaderID)
	} else {
		log.Printf("ServerNode %d elected as the new leader.", newLeaderID)
	}
	n.leaderID = newLeaderID

	// Notify all neighbors about the new leader
	for _, neighbor := range n.neighbors {
		status := &proto.Status{ServerId: n.leaderID}
		_, err := neighbor.ExecuteElection(context.Background(), status)
		if err != nil {
			log.Printf("Failed to notify neighbor about new leader: %v", err)
		}
	}
}

// getLeaderClient retrieves the gRPC client for the current leader.
func (n *ServerNode) getLeaderClient() (proto.AuctionServiceClient, error) {
	if n.leaderID == n.uniqueID {
		return nil, fmt.Errorf("current node is the leader")
	}

	index, exists := n.nodeIDToIndex[n.leaderID]
	if !exists || index >= len(n.neighbors) {
		return nil, fmt.Errorf("leader client not found")
	}
	return n.neighbors[index], nil
}

// getCurrentHighestBid returns the highest bid value.
func (n *ServerNode) getCurrentHighestBid() int32 {
	var highest int32
	for _, bid := range n.bidsHistory {
		if bid > highest {
			highest = bid
		}
	}
	return highest
}

// getCurrentHighestBidder returns the ID of the bidder with the highest bid.
func (n *ServerNode) getCurrentHighestBidder() int32 {
	var highestBidder int32
	var highestBid int32
	for bidder, bid := range n.bidsHistory {
		if bid > highestBid {
			highestBid = bid
			highestBidder = bidder
		}
	}
	return highestBidder
}

// simulateCrash randomly simulates a node crash with a 30% probability.
func (n *ServerNode) simulateCrash() {
	time.Sleep(time.Duration(20+rand.Intn(20)) * time.Second)
	if rand.Intn(100) < 30 {
		log.Printf("Simulating crash of ServerNode %d", n.uniqueID)
		// Exit the process to simulate a crash
		// In a real system, you'd handle graceful shutdowns
		// For simulation purposes, we'll just log and exit
		log.Fatal("ServerNode crashed.")
	}
}

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Usage: %s <port> <node1_ip:node1_port> <node2_ip:node2_port> ...", os.Args[0])
	}

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("Invalid port number: %v", err)
	}

	nodeAddresses := os.Args[2:]
	ip := "localhost" // Assuming all nodes are on localhost; adjust as needed

	node := NewServerNode(ip, port)
	node.Start(nodeAddresses)
}
