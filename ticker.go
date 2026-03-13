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
	quit     chan struct{}
	running  bool
}

// NewTicker creates the Ticker structure and quit channel.
func NewTicker(d time.Duration) *Ticker {
	t := &Ticker{
		duration: d,
		funcs:    make(map[int64]EventFunc),
		quit:     make(chan struct{}, 1),
	}
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
	if t.running {
		t.Unlock()
		return
	}
	t.running = true
	t.ticker = time.NewTicker(t.duration)
	// mark supervisor goroutine
	t.Add(1)
	go func() {
		defer t.Add(-1)
		for {
			select {
			case <-t.ticker.C:
				// snapshot funcs under read lock to avoid holding lock while calling
				t.RLock()
				funcs := make([]EventFunc, 0, len(t.funcs))
				for _, f := range t.funcs {
					funcs = append(funcs, f)
				}
				t.RUnlock()
				for _, f := range funcs {
					t.Add(1)
					go func(tf EventFunc) {
						defer t.Done()
						tf(0)
					}(f)
				}
			case <-t.quit:
				t.ticker.Stop()
				// clear funcs map
				t.Lock()
				for k := range t.funcs {
					delete(t.funcs, k)
				}
				t.running = false
				t.Unlock()
				return
			}
		}
	}()
	t.Unlock()
}

// Stop the ticker.
func (t *Ticker) Stop() {
	// signal quit non-blocking
	select {
	case t.quit <- struct{}{}:
	default:
	}
	// wait for supervisor and handlers
	t.Wait()
}
