package gon

import (
	"sync"
	"time"
)

// Ticker runs one or more functions, repeating at an interval.
type Ticker struct {
	sync.RWMutex
	sync.WaitGroup
	duration time.Duration
	ticker   *time.Ticker
	funcs    map[int64]EventFunc
	quit     chan bool
	running  bool
}

// NewTicker creates the Ticker structure and quit channel.
func NewTicker(d time.Duration) *Ticker {
	t := &Ticker{
		duration: d,
		funcs:    make(map[int64]EventFunc)}
	t.quit = make(chan bool)
	return t
}

// AddFunc adds another callback to the funcs map with a new ID.
func (t *Ticker) AddFunc(f EventFunc, id int64) {
	t.Lock()
	defer t.Unlock()
	t.funcs[id] = f
}

// Start creates the time.Ticker and handles the calls at intervals.
func (t *Ticker) Start() {
	if t.running {
		return
	}

	t.Lock()
	defer t.Unlock()
	t.running = true
	t.ticker = time.NewTicker(t.duration)
	go func() {
		for {
			select {
			case <-t.ticker.C:
				for k, f := range t.funcs {
					t.Add(1)
					go func(id int64, tf EventFunc) {
						tf(id)
						t.Done()
					}(k, f)
				}
			case <-t.quit:
				t.ticker.Stop()
				for k := range t.funcs {
					delete(t.funcs, k)
				}
				t.running = false
				return
			}
		}
	}()
}

// Stop the ticker.
func (t *Ticker) Stop() {
	if !t.running {
		return
	}

	t.quit <- true
	t.Wait()
}
