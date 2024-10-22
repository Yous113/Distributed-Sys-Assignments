package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"os"
	"io"

	pb "assignment3/proto"

	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedChittyChatServer
	mu          sync.Mutex
	clients     map[string]pb.ChittyChat_BroadcastServer
	lamportTime int32
	messages    []*pb.BroadcastMessage // Slice to store all messages
}

func newServer() *server {
	return &server{
		clients:     make(map[string]pb.ChittyChat_BroadcastServer),
		lamportTime: 0,
		messages:    []*pb.BroadcastMessage{}, // Initialize the messages slice
	}
}

// JoinChat allows a client to join the chat and receive broadcast messages
func (s *server) JoinChat(ctx context.Context, req *pb.JoinChatRequest) (*pb.JoinChatResponse, error) {
	participantName := req.GetParticipantName()

	s.mu.Lock()
	s.lamportTime++
	currentLamportTime := s.lamportTime
	s.mu.Unlock()

	log.Printf("Participant %s joined at Lamport time %d", participantName, currentLamportTime)

	return &pb.JoinChatResponse{
		Success:     true,
		LamportTime: currentLamportTime,
	}, nil
}

// Broadcast handles bidirectional streaming between server and clients
// Broadcast handles bidirectional streaming between server and clients
func (s *server) Broadcast(stream pb.ChittyChat_BroadcastServer) error {
    // Receive the first message to get participantName
    req, err := stream.Recv()
    if err != nil {
        return err
    }
    participantName := req.GetParticipantName()

    s.mu.Lock()
    s.lamportTime++
    currentLamportTime := s.lamportTime
    s.clients[participantName] = stream
    s.mu.Unlock()

    // Broadcast join message
    joinMessage := &pb.BroadcastMessage{
        ParticipantName: participantName,
        Message:         fmt.Sprintf("Participant %s joined Chitty-Chat at Lamport time %d", participantName, currentLamportTime),
        LamportTime:     currentLamportTime,
    }
    s.broadcastMessage(joinMessage)

    // Handle incoming messages from the client
    for {
        req, err := stream.Recv()
        if err != nil {
            // Client disconnected
            break
        }

        s.mu.Lock()
        s.lamportTime++
        currentLamportTime := s.lamportTime
        s.mu.Unlock()

        // Log the received message
        log.Printf("Received message at Lamport time %d from %s %s", currentLamportTime, participantName, req.GetMessage())

        // Broadcast the chat message to all clients
        chatMessage := &pb.BroadcastMessage{
            ParticipantName: participantName,
            Message:         req.GetMessage(),
            LamportTime:     currentLamportTime,
        }
        s.broadcastMessage(chatMessage)
    }

    // Handle client disconnect
    s.mu.Lock()
    delete(s.clients, participantName)
    s.lamportTime++
    currentLamportTime = s.lamportTime
    s.mu.Unlock()

    // Broadcast leave message
    leaveMessage := &pb.BroadcastMessage{
        ParticipantName: participantName,
        Message:         fmt.Sprintf("Participant %s left Chitty-Chat at Lamport time %d", participantName, currentLamportTime),
        LamportTime:     currentLamportTime,
    }
    s.broadcastMessage(leaveMessage)

    return nil
}

// broadcastMessage sends the message to all connected clients and stores it in the messages slice
func (s *server) broadcastMessage(message *pb.BroadcastMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store the message in the messages slice
	s.messages = append(s.messages, message)

	for participantName, clientStream := range s.clients {
		go func(name string, stream pb.ChittyChat_BroadcastServer) {
			if err := stream.Send(message); err != nil {
				log.Printf("Error sending message to %s: %v", name, err)
			}
		}(participantName, clientStream)
	}
}

func main() {
	logfile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logfile.Close()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	MultiWriter := io.MultiWriter(logfile, os.Stdout)
	log.SetOutput(MultiWriter)

	grpcServer := grpc.NewServer()
	pb.RegisterChittyChatServer(grpcServer, newServer())

	log.Println("Chitty-Chat server is running on port 50051...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}