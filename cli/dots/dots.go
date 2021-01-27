package dots

import (
	"fmt"
	"time"
)

// Dots is a console writer writing dots periodically
type Dots struct {
	c        chan bool
	WaitTime time.Duration
}

// New create dot dots.
func New(waitTime uint) Dots {
	return Dots{
		c:        make(chan bool),
		WaitTime: time.Duration(waitTime),
	}
}

// Start starts the dot dots
func (d *Dots) Start() {
	go d.run()
}

// Stop stop the dots
func (d *Dots) Stop() {
	d.c <- true
}

func (d *Dots) run() {
	for {
		select {
		case <-d.c:
			break
		case <-time.After(time.Second * d.WaitTime):
			fmt.Print(".")
		}
	}
}
