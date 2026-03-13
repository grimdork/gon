package gon

import (
	"sync"
	"time"
)

// Alarm runs a single function once.
type Alarm struct {
	sync.RWMutex
	sync.WaitGroup
	scheduler *Scheduler
	delay     time.Duration
	timer     *time.Timer
	id        int64
	f         EventFunc
	quit      chan struct{}
	running   bool
}

// NewAlarm creates the Alarm structure and quit channel.
func NewAlarm(d time.Duration, aid int64, af EventFunc) *Alarm {
	a := &Alarm{
		delay: d,
		id:    aid,
		f:     af,
		quit:  make(chan struct{}, 1),
	}
	return a
}

// Start creates a one-shot timer and runs it after its duration has passed.
// If the alarm is a repeating alarm, a 24-hour Ticker is created.
func (a *Alarm) Start() {
	if a.running {
		return
	}

	a.Lock()
	if a.running {
		a.Unlock()
		return
	}
	a.running = true
	if a.timer == nil {
		a.timer = time.NewTimer(a.delay)
	} else {
		a.timer.Reset(a.delay)
	}
	// mark the supervisor goroutine in the WaitGroup
	a.Add(1)
	a.Unlock()

	go func() {
		defer a.Add(-1)
		for {
			select {
			case <-a.timer.C:
				// launch the handler
				a.Add(1)
				go func(id int64, af EventFunc) {
					defer a.Done()
					af(id)
				}(a.id, a.f)
				// remove from scheduler without calling back into Stop
				if a.scheduler != nil {
					a.scheduler.RemoveAlarm(a.id)
				}
				return
			case <-a.quit:
				// stop timer and exit; do not call Done here for handler WaitGroup
				if !a.timer.Stop() {
					// drain timer channel if needed
					select {
					case <-a.timer.C:
					default:
					}
				}
				return
			}
		}
	}()
}

// Stop stops the alarm and waits for any in-flight handlers to complete.
// It does NOT call back into the scheduler to remove the alarm; callers
// (typically Scheduler.RemoveAlarm) should coordinate removal from the map.
func (a *Alarm) Stop() {
	a.Lock()
	if !a.running {
		a.Unlock()
		return
	}
	// signal quit without blocking
	select {
	case a.quit <- struct{}{}:
	default:
	}
	a.running = false
	a.Unlock()
	// wait for supervisor goroutine and any handler goroutines to finish
	a.Wait()
}
