package gon

import (
	"sync"
	"time"
)

// Alarm runs a single function once.
type Alarm struct {
	sync.RWMutex
	scheduler *Scheduler
	sync.WaitGroup
	delay   time.Duration
	timer   *time.Timer
	id      int64
	f       EventFunc
	quit    chan bool
	repeat  bool
	running bool
}

// NewAlarm creates the Alarm structure and quit channel.
func NewAlarm(d time.Duration, aid int64, af EventFunc) *Alarm {
	a := &Alarm{
		delay: d,
		id:    aid,
		f:     af,
	}
	a.quit = make(chan bool)
	return a
}

// Start creates a one-shot timer and runs it after its duration has passed.
// If the alarm is a repeating alarm, a 24-hour Ticker is created.
func (a *Alarm) Start() {
	if a.running {
		return
	}

	a.running = true
	a.timer = time.NewTimer(a.delay)
	for {
		select {
		case <-a.timer.C:
			a.Lock()
			a.Add(1)
			go func(id int64, af EventFunc) {
				af(id)
				a.Done()
			}(a.id, a.f)
			a.Unlock()
			a.Wait()
			if a.repeat {
				// The alarm transitions to repeating ticker here
				a.scheduler.Repeat(24*time.Hour, a.f)
			}
			a.scheduler.RemoveAlarm(a.id)
			return
		case <-a.quit:
			a.timer.Stop()
			a.running = false
			return
		}
	}
}

// Stop and remove the alarm.
func (a *Alarm) Stop() {
	a.Wait()
	a.quit <- true
	a.scheduler.RemoveAlarm(a.id)
}
