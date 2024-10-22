package main

import (
    "bufio"
    "context"
    "io"
    "log"
    "os"
    "strings"
    "sync"
    "time"

    pb "assignment3/proto"

    "google.golang.org/grpc"
)

// Custom client struct to track participant and Lamport time
type chatClient struct {
    participantName string
    lamportTime     int32
    mu              sync.Mutex
}

func max(a, b int32) int32 {
    if a > b {
        return a
    }
    return b
}

// Updates Lamport time based on received message's Lamport time
func (c *chatClient) updateLamportTime(receivedTime int32) {
    c.mu.Lock()
    c.lamportTime = max(c.lamportTime, receivedTime) + 1
    c.mu.Unlock()
}

func main() {
	logfile, err := os.OpenFile("client.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if len(os.Args) < 2 {
        log.Fatalf("Usage: go run client.go [participantName]")
    }


    participantName := os.Args[1]

	MultiWriter := io.MultiWriter(logfile, os.Stdout)
	log.SetOutput(MultiWriter)
    // Connect to the gRPC server
    conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to server: %v", err)
    }
    defer conn.Close()


    chittyChatClient := pb.NewChittyChatClient(conn)

    // Initialize the custom client
    client := &chatClient{
        participantName: participantName,
        lamportTime:     0,
    }

    // Join the chat
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
    defer cancel()

    joinResponse, err := chittyChatClient.JoinChat(ctx, &pb.JoinChatRequest{
        ParticipantName: participantName,
    })
    if err != nil {
        log.Fatalf("Failed to join chat: %v", err)
    }

    client.mu.Lock()
    client.lamportTime = max(client.lamportTime, joinResponse.GetLamportTime()) + 1
    currentLamportTime := client.lamportTime
    client.mu.Unlock()

    log.Printf("Joined chat as %s at Lamport time %d", participantName, currentLamportTime)

    // Start bidirectional streaming
    stream, err := chittyChatClient.Broadcast(context.Background())
    if err != nil {
        log.Fatalf("Failed to start broadcast: %v", err)
    }

    // Send initial message to register with the server
    err = stream.Send(&pb.BroadcastMessageRequest{
        ParticipantName: participantName,
    })
    if err != nil {
        log.Fatalf("Failed to send initial broadcast message: %v", err)
    }

    // Start two goroutines: one for receiving messages, one for sending
    waitGroup := &sync.WaitGroup{}
    waitGroup.Add(2)

    // Goroutine for receiving messages
    go func() {
        defer waitGroup.Done()
        for {
            msg, err := stream.Recv()
            if err == io.EOF {
                break
            }
            if err != nil {
                log.Printf("Error receiving message: %v", err)
                break
            }

            // Update Lamport time upon receiving a message
            client.updateLamportTime(msg.GetLamportTime())

            // Log the received message and Lamport time
            log.Printf("[Lamport Time %d] %s: %s", client.lamportTime, msg.GetParticipantName(), msg.GetMessage())
        }
    }()

    // Goroutine for sending messages
    go func() {
        defer waitGroup.Done()
        scanner := bufio.NewScanner(os.Stdin)
        for scanner.Scan() {
            text := scanner.Text()
            if strings.TrimSpace(text) == "" {
                continue
            }

            // Update Lamport time before sending
            client.mu.Lock()
            client.lamportTime++
            currentLamportTime := client.lamportTime
            client.mu.Unlock()

            // Send the message to the server
            err := stream.Send(&pb.BroadcastMessageRequest{
                ParticipantName: participantName,
                Message:         text,
                LamportTime:     currentLamportTime,
            })
            if err != nil {
                log.Printf("Error sending message: %v", err)
                break
            }

			log.Printf("%s sending message at Lamport Time %d", participantName, currentLamportTime)
        }

        if err := scanner.Err(); err != nil {
            log.Printf("Error reading from stdin: %v", err)
        }
    }()

    // Wait for both goroutines to finish
    waitGroup.Wait()

    // Leave the chat when done
    leaveResponse, err := chittyChatClient.LeaveChat(context.Background(), &pb.LeaveChatRequest{
        ParticipantName: participantName,
    })
    if err != nil {
        log.Printf("Error leaving chat: %v", err)
    } else {
        client.updateLamportTime(leaveResponse.GetLamportTime())
        log.Printf("Left chat at Lamport time %d", client.lamportTime)
    }
}
