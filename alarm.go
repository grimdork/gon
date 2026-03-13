package gon

import (
	"context"
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
	// context-aware callback
	f       EventFuncCtx
	quit    chan struct{}
	running bool
	// cancel func for in-flight handler
	handlerCancel context.CancelFunc
}

// NewAlarm creates the Alarm structure and quit channel. Accepts the old
// EventFunc signature and adapts it to a context-aware callback.
func NewAlarm(d time.Duration, aid int64, af EventFunc) *Alarm {
	return NewAlarmCtx(d, aid, func(ctx context.Context, id int64) {
		// adapter: ignore context
		af(id)
	})
}

// NewAlarmCtx creates an Alarm with a context-aware callback.
func NewAlarmCtx(d time.Duration, aid int64, af EventFuncCtx) *Alarm {
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
				// create cancellable context for handler
				ctx, cancel := context.WithCancel(context.Background())
				a.Lock()
				a.handlerCancel = cancel
				a.Unlock()
				go func(id int64, af EventFuncCtx) {
					defer a.Done()
					af(ctx, id)
					// clear cancel after handler finishes
					a.Lock()
					a.handlerCancel = nil
					a.Unlock()
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
	// cancel handler if running
	if a.handlerCancel != nil {
		a.handlerCancel()
	}
	a.running = false
	a.Unlock()
	// wait for supervisor goroutine and any handler goroutines to finish
	a.Wait()
}
