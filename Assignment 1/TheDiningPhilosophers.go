package main

import (
	"fmt"
	"sync"
	"time"
)

// Fork struct represents a fork with an ID and communication channels
type Fork struct {
	id          int
	acquireChan chan chan bool // Channel to receive acquisition requests from philosophers
	releaseChan chan bool      // Channel to receive release signals from philosophers
}

// start method runs the fork's goroutine
func (f *Fork) start() {
	for {
		// Wait for a philosopher to request the fork
		replyChan := <-f.acquireChan
		// Fork is now taken
		replyChan <- true // Acknowledge to the philosopher that the fork has been acquired

		// Wait for the philosopher to release the fork
		<-f.releaseChan
		// Fork is now free and can accept new acquisition requests
	}
}

// Philosopher struct represents a philosopher with an ID and references to left and right forks
type Philosopher struct {
	id                  int
	leftFork, rightFork *Fork
}

// dine method simulates the philosopher's dining process
func (p Philosopher) dine(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		p.think()
		p.eat()
	}
}

func (p Philosopher) think() {
	fmt.Printf("Philosopher %d is thinking...\n", p.id)
	time.Sleep(time.Second)
}

func (p Philosopher) eat() {
	// Create acknowledgment channels for fork acquisition
	leftAck := make(chan bool)
	rightAck := make(chan bool)

	// To avoid deadlock, odd philosophers pick up left fork first, even pick up right fork first
	// This breaks the circular wait condition necessary for deadlock
	if p.id%2 == 0 {
		// Even philosopher picks up right fork first
		p.rightFork.acquireChan <- rightAck
		<-rightAck // Wait for acknowledgment

		p.leftFork.acquireChan <- leftAck
		<-leftAck // Wait for acknowledgment
	} else {
		// Odd philosopher picks up left fork first
		p.leftFork.acquireChan <- leftAck
		<-leftAck

		p.rightFork.acquireChan <- rightAck
		<-rightAck
	}

	fmt.Printf("Philosopher %d is eating...\n", p.id)
	time.Sleep(time.Second)

	// Release the forks
	p.leftFork.releaseChan <- true
	p.rightFork.releaseChan <- true
}

func main() {
	var wg sync.WaitGroup
	numPhilosophers := 5

	// Initialize forks
	forks := make([]*Fork, numPhilosophers)
	for i := 0; i < numPhilosophers; i++ {
		forks[i] = &Fork{
			id:          i + 1,
			acquireChan: make(chan chan bool),
			releaseChan: make(chan bool),
		}
		// Start the fork's goroutine
		go forks[i].start()
	}

	// Initialize philosophers
	philosophers := make([]Philosopher, numPhilosophers)
	for i := 0; i < numPhilosophers; i++ {
		philosophers[i] = Philosopher{
			id:        i + 1,
			leftFork:  forks[i],
			rightFork: forks[(i+1)%numPhilosophers],
		}
		wg.Add(1)
		go philosophers[i].dine(&wg)
	}

	wg.Wait()
}
