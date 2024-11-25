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
	"io"
	proto "cookAssignment5/proto"

	"google.golang.org/grpc"
)

type AuctionClient struct {
	clientID int64
	bidValue int
	server   proto.AuctionServiceClient
}

var (
	clientIDFlag = flag.Int64("id", 0, "Unique ID for the client")
)

func main() {

	logfile, err := os.OpenFile("client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logfile.Close()

	multiWriter := io.MultiWriter(logfile, os.Stdout)
	log.SetOutput(multiWriter)
	flag.Parse()

	client := AuctionClient{
		clientID: *clientIDFlag,
		bidValue: 0,
	}

	client.connectToAuctionServer()
	client.startAuctionInterface()
}

func (c *AuctionClient) connectToAuctionServer() {
	file, err := os.Open("../nodes.txt")
	if err != nil {
		log.Fatalf("Unable to read node configuration file: %v", err)
	}
	defer file.Close()

	var nodeList []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		nodeList = append(nodeList, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading node configuration: %v", err)
	}

	connectionEstablished := false
	for !connectionEstablished {
		randomNode := nodeList[rand.Intn(len(nodeList))]
		nodeDetails := strings.Split(randomNode, " ")
		ip := nodeDetails[0]
		port, _ := strconv.Atoi(nodeDetails[1])

		conn, err := grpc.Dial(ip+":"+strconv.Itoa(port), grpc.WithInsecure())
		if err == nil {
			log.Printf("Connected to auction server at port %d\n", port)
			c.server = proto.NewAuctionServiceClient(conn)
			connectionEstablished = true
		}
	}
}

func (c *AuctionClient) placeBid(amount int) {
	if amount > c.bidValue {
		c.bidValue = amount

		for {
			_, err := c.server.PlaceBid(context.Background(), &proto.Bid{
				ClientId:     c.clientID,
				BidAmount:    int64(amount),
				IsFromLeader: false,
			})

			if err != nil {
				log.Println("Failed to place bid. Reconnecting...")
				c.connectToAuctionServer()
			} else {
				log.Printf("Successfully placed bid: %d\n", amount)
				break
			}
		}
	} else {
		fmt.Println("Your bid must be higher than the current value.")
	}
}

func (c *AuctionClient) fetchAuctionResults() {
	for {
		result, err := c.server.FetchResult(context.Background(), &proto.Empty{})
		if err != nil {
			log.Println("Error fetching results. Reconnecting...")
			c.connectToAuctionServer()
		} else {
			log.Println("Auction Result:", result.ResultMessage)
			break
		}
	}
}

func (c *AuctionClient) startAuctionInterface() {
	fmt.Println("Enter a bid amount to place a bid.")
	fmt.Println("'result' to view the current auction result.")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		if strings.ToLower(input) == "result" {
			go c.fetchAuctionResults()
		} else {
			bidAmount, err := strconv.Atoi(input)
			if err != nil {
				fmt.Println("Invalid input. Please enter a number or 'result'.")
			} else {
				go c.placeBid(bidAmount)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
}
