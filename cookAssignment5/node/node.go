package auction

import (
	"context"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	proto "cookAssignment5/proto"

	"google.golang.org/grpc"
)

type AuctionNode struct {
	proto.UnimplementedAuctionServiceServer
	address         string
	nodePort        int
	nodeID          int64
	currentLeader   int64
	neighborIndex   int
	peers           []proto.AuctionServiceClient
	bidMap          map[int64]int64
	isAuctionActive bool
	lock            sync.Mutex
	portToIndexMap  map[int]int
}

func InitializeNode(ip string, port int) *AuctionNode {
	logFile, err := os.OpenFile("nodes.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	log.SetOutput(logFile)

	log.Printf("Initializing node with IP: %s and Port: %d", ip, port)
	return &AuctionNode{
		address:        ip,
		nodePort:       port,
		nodeID:         int64(port),
		bidMap:         make(map[int64]int64),
		portToIndexMap: make(map[int]int),
	}
}

func (node *AuctionNode) Launch(peerAddresses []string) {
	log.Println("Launching node...")
	go node.setupServer()

	time.Sleep(10 * time.Second)

	for _, peer := range peerAddresses {
		ipAndPort := strings.Split(peer, " ")
		port, _ := strconv.Atoi(ipAndPort[1])
		if port != node.nodePort {
			client, err := node.connectToPeer(ipAndPort[0], port)
			if err != nil {
				log.Fatalf("Failed to connect to peer: %v", err)
			}
			node.peers = append(node.peers, client)
		} else {
			node.neighborIndex = len(node.peers)
			node.peers = append(node.peers, nil)
		}
		node.portToIndexMap[port] = len(node.portToIndexMap)
	}

	node.isAuctionActive = true

	if node.peers[0] == nil {
		node.StartElection(context.Background(), &proto.ElectionStatus{ServerId: node.nodeID})
	}

	go node.manageAuction()
}

func (node *AuctionNode) manageAuction() {
	log.Println("Auction started.")
	time.Sleep(60 * time.Second)
	node.isAuctionActive = false
	log.Println("Auction ended. No longer active.")

	select {}
}

func (node *AuctionNode) PlaceBid(ctx context.Context, bid *proto.Bid) (*proto.Ack, error) {
	log.Printf("Received bid: ClientID=%d, Amount=%d, FromLeader=%v", bid.ClientId, bid.BidAmount, bid.IsFromLeader)
	if node.isAuctionActive {
		if node.currentLeader == node.nodeID {
			log.Printf("Processing bid as leader: ClientID=%d, Amount=%d", bid.ClientId, bid.BidAmount)
			node.recordBid(bid.ClientId, bid.BidAmount)
			bid.IsFromLeader = true
			for _, peer := range node.peers {
				if peer != nil {
					_, err := peer.PlaceBid(ctx, bid)
					if err != nil {
						log.Printf("Error propagating bid to peer: %v", err)
					}
				}
			}
		} else if !bid.IsFromLeader {
			log.Printf("Forwarding bid to leader: LeaderID=%d", node.currentLeader)
			leaderIndex := node.portToIndexMap[int(node.currentLeader)]
			_, err := node.peers[leaderIndex].PlaceBid(ctx, bid)
			if err != nil {
				log.Printf("Failed to forward bid to leader. Triggering election.")
				node.StartElection(ctx, &proto.ElectionStatus{ServerId: node.nodeID})
				return node.PlaceBid(ctx, bid) // Retry after election
			}
		} else {
			log.Printf("Processing forwarded bid: ClientID=%d, Amount=%d", bid.ClientId, bid.BidAmount)
			node.recordBid(bid.ClientId, bid.BidAmount)
		}
	} else {
		log.Printf("Bid rejected. Auction is not active.")
	}
	return &proto.Ack{}, nil
}

func (node *AuctionNode) InitiateLeaderElection(ctx context.Context, bid *proto.Bid) (*proto.Ack, error) {
	if node.isAuctionActive {
		if node.currentLeader == node.nodeID {
			// Leader processes the bid and propagates it to peers
			node.recordBid(bid.ClientId, bid.BidAmount)
			bid.IsFromLeader = true
			for _, peer := range node.peers {
				if peer != nil {
					_, err := peer.PlaceBid(ctx, bid)
					if err != nil {
						log.Printf("Error propagating bid to peer: %v", err)
					}
				}
			}
		} else if !bid.IsFromLeader {
			// Forward bid to the leader
			leaderIndex := node.portToIndexMap[int(node.currentLeader)]
			_, err := node.peers[leaderIndex].PlaceBid(ctx, bid)
			if err != nil {
				log.Printf("Failed to forward bid to leader. Triggering election.")
				node.StartElection(ctx, &proto.ElectionStatus{ServerId: node.nodeID})
				return node.PlaceBid(ctx, bid) // Retry after election
			}
		} else {
			// Non-leader node processes the forwarded bid
			node.recordBid(bid.ClientId, bid.BidAmount)
		}
	} else {
		log.Printf("Bid rejected. Auction is not active.")
	}
	return &proto.Ack{}, nil
}

func (node *AuctionNode) StartElection(ctx context.Context, status *proto.ElectionStatus) (*proto.Empty, error) {
	if status.ServerId == node.nodeID {
		node.currentLeader = status.ServerId
		log.Println("Election concluded. Leader:", node.currentLeader)
		return &proto.Empty{}, nil
	} else if status.ServerId < node.nodeID {
		status.ServerId = node.nodeID
	}
	node.currentLeader = status.ServerId

	for {
		_, err := node.peers[node.neighborIndex].StartElection(ctx, status)
		if err != nil {
			node.neighborIndex = (node.neighborIndex - 1 + len(node.peers)) % len(node.peers)
			if node.peers[node.neighborIndex] == nil {
				node.currentLeader = node.nodeID
				break
			}
		} else {
			break
		}
	}
	return &proto.Empty{}, nil
}

func (node *AuctionNode) FetchResult(ctx context.Context, _ *proto.Empty) (*proto.Result, error) {
	if node.currentLeader == node.nodeID {
		if len(node.bidMap) == 0 {
			return &proto.Result{ResultMessage: "No bids submitted."}, nil
		}
		highestBidder := int64(0)
		highestBid := int64(0)
		for bidder, bid := range node.bidMap {
			if bid > highestBid {
				highestBid = bid
				highestBidder = bidder
			}
		}
		status := "Auction is ongoing."
		if !node.isAuctionActive {
			status = "Auction concluded."
		}
		log.Printf("Result computed: Bidder=%d, Amount=%d", highestBidder, highestBid)
		return &proto.Result{
			ResultMessage: status + " Highest bidder: " + strconv.Itoa(int(highestBidder)) + " with bid: " + strconv.Itoa(int(highestBid)),
		}, nil
	}
	log.Printf("Forwarding FetchResult to leader: LeaderID=%d", node.currentLeader)
	return node.peers[node.portToIndexMap[int(node.currentLeader)]].FetchResult(ctx, nil)
}

func (node *AuctionNode) recordBid(bidderID, amount int64) {
	node.lock.Lock()
	defer node.lock.Unlock()
	node.bidMap[bidderID] = amount
}

func (node *AuctionNode) setupServer() {
	grpcServer := grpc.NewServer()

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(node.nodePort))
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
	log.Printf("Server running on port %d\n", node.nodePort)

	proto.RegisterAuctionServiceServer(grpcServer, node)

	if rand.Intn(100) < 30 {
		go node.simulateCrash(grpcServer)
	}

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Println("Server error:", err)
	}
}

func (node *AuctionNode) connectToPeer(ip string, port int) (proto.AuctionServiceClient, error) {
	conn, err := grpc.Dial(ip+":"+strconv.Itoa(port), grpc.WithInsecure())
	return proto.NewAuctionServiceClient(conn), err
}

func (node *AuctionNode) simulateCrash(server *grpc.Server) {
	time.Sleep(time.Duration(20+rand.Intn(20)) * time.Second)
	log.Printf("Node crashed: %d\n", node.nodePort)
	server.Stop()
}
