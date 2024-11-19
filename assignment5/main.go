package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"

	servernode "assignment5/ServerNode"
)

func main() {
	file, err := os.Open("Servernodes.txt")
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var nodes []string

	for scanner.Scan() {
		nodes = append(nodes, scanner.Text())
	}

	for _, node := range nodes {
		parts := strings.Split(node, " ")
		ip := parts[0]
		port, _ := strconv.Atoi(parts[1])

		newNode := servernode.NewServerNode(ip, port)
		go newNode.Start(nodes)
	}

	select {}
}
