package main

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"strings"
)

func main() {
	server()
}

func server() {
	RandomNumber2 := rand.Intn(1000)
	// Listen on a TCP port
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer ln.Close() // Ensure the listener is closed when the server exits

	for {
		// Accept a connection
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}
		go handleConnection(conn, RandomNumber2) // Handle the connection in a new goroutine
	}
}

func handleConnection(conn net.Conn, seq int) {
	defer conn.Close() // Ensure the connection is closed when done

	for {
		// Read from client
		buffer := make([]byte, 1024) // Create a buffer to hold the data
		incomingData, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Client closed the connection.")
				return
			}
			fmt.Println("Error reading:", err)
			return
		}

		var data = string(buffer[:incomingData])
		var slice = strings.Split(data, " ")

		if slice[0] == "SYN" {
			number := slice[1]
			ack, _ := strconv.Atoi(number)
			fmt.Printf("Received SYN seq = %d from client\n", ack)
			ack++

			ackStr := strconv.Itoa(ack)
			seqStr := strconv.Itoa(seq)

			ar := []string{"SYN-ACK", ackStr, seqStr}
			message := strings.Join(ar, " ")
			fmt.Printf("Sending SYN-ACK ack = %d seq = %s to client\n", ack, seqStr)
			conn.Write([]byte(message))
		}

		if slice[0] == "ACK" {
			number := slice[1]
			numberOfSEQ := slice[2]
			ack, _ := strconv.Atoi(number)
			seqq, _ := strconv.Atoi(numberOfSEQ)
			if (ack - 1) == seq {
				seq++
				fmt.Printf("Received ACK ack = %d seq = %d received from client\n", ack, seqq)
				fmt.Println("Connection established")
				return // Exit the handler after successful handshake
			} else {
				fmt.Println("Connection failed")
				return // Exit the handler on failure
			}
		}
	}
}
