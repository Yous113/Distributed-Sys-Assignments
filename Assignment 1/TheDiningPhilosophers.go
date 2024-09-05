package main

import (
	"fmt"
	"sync"
	"time"
)

// Philosopher struct represents a philosopher with an ID and two forks (channels)
// Channels are used to communicate between philosophers and forks to know if a fork is available
type Philosopher struct {
	id                  int
	leftFork, rightFork chan bool
}

// dine method simulates the philosopher's dining process
func (p Philosopher) dine(wg *sync.WaitGroup) {
	// Notify the WaitGroup that this goroutine is done when the function returns
	defer wg.Done()
	for {
		p.think()
		p.eat()
	}
}

func fork(forkChan chan bool) {
	for {
		<-forkChan
		forkChan <- true
	}
}

func (p Philosopher) think() {
	fmt.Printf("Philosopher %d is thinking...\n", p.id)
	time.Sleep(time.Second)
}

func (p Philosopher) eat() {
	// To avoid deadlock
	if p.id%2 == 0 {
		p.leftFork <- true
		p.rightFork <- true
	} else {
		p.rightFork <- true
		p.leftFork <- true
	}

	fmt.Printf("Philosopher %d is eating...\n", p.id)
	go fork(p.leftFork)
	go fork(p.rightFork)

	<-p.leftFork
	<-p.rightFork
}

func main() {
	var wg sync.WaitGroup
	forks := make([]chan bool, 5)
	philosophers := make([]Philosopher, 5)

	for i := 0; i < 5; i++ {
		forks[i] = make(chan bool, 1)
	}

	for i := 0; i < 5; i++ {
		philosophers[i] = Philosopher{
			id:        i + 1,
			leftFork:  forks[i],
			rightFork: forks[(i+1)%5],
		}
		wg.Add(1)
		go philosophers[i].dine(&wg)
	}

	wg.Wait()
}
