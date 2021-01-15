package waiter

import (
	"fmt"
	"time"
)

type Waiter struct {
	c        chan bool
	WaitTime time.Duration
}

// New create dot waiter.
func New(waitTime uint) Waiter {
	return Waiter{
		c:        make(chan bool),
		WaitTime: time.Duration(waitTime),
	}
}

// Start starts the dot waiter
func (w *Waiter) Start() {
	go w.run()
}

// Stop stop the dot waiter
func (w *Waiter) Stop() {
	w.c <- true
}

func (w *Waiter) run() {
	for {
		select {
		case <-w.c:
			break
		case <-time.After(time.Second * w.WaitTime):
			fmt.Print(".")
		}
	}
}
