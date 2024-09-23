package main

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

func main() {
	client()
}

func client() {
	seq := 100
	stringC := strconv.Itoa(seq)

	ar := []string{"SYN", stringC}
	array := strings.Join(ar, " ")

	// Dial the server
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}

	fmt.Printf("Sending SYN seq %d to server\n", seq)
	conn.Write([]byte(array))

	// Read data from the server
	buffer := make([]byte, 1024)           // Create a buffer to hold the data
	ReceivedData, err := conn.Read(buffer) // Read the data into the buffer
	if err != nil && err != io.EOF {
		fmt.Println("Error reading:", err)
		return
	}

	var data = string(buffer[:ReceivedData])

	var slice = strings.Split(data, " ")

	if slice[0] == "SYN-ACK" {
		numberSEQ := slice[2]
		numberaACK := slice[1]
		seqN, _ := strconv.Atoi(numberaACK)

		if seq == seqN-1 {
			seq++
			ack, _ := strconv.Atoi(numberSEQ)
			fmt.Printf("Received SYN-ACK ack = %s seq = %d from server\n", numberaACK, ack)
			ack++

			ackStr := strconv.Itoa(ack)

			ar := []string{"ACK", ackStr, numberaACK}
			message := strings.Join(ar, " ")
			fmt.Printf("Sending ACK ack = %d seq = %s to server\n", ack, numberaACK)
			conn.Write([]byte(message))
		} else {
			fmt.Println("Error: Sequence number does not match")
		}

	}
}
