package gon

import (
	"context"
	"sync"
	"time"
)

// Scheduler holds pointers to all the tickers and timers.
type Scheduler struct {
	sync.RWMutex
	id             int64
	jobs           map[int64]*tickerEntry
	tickers        map[time.Duration]*Ticker
	dormantTickers map[time.Duration]*Ticker
	alarms         map[int64]*Alarm
}

// tickerEntry records a job id and its duration group.
type tickerEntry struct {
	id       int64
	duration time.Duration
}

// EventFunc is the legacy signature of the ticker/alarm callbacks.
// Use EventFuncCtx for context-aware handlers.
type EventFunc func(int64)

// EventFuncCtx is the newer context-aware callback signature.
// Callers can provide either; the scheduler/ticker adapts legacy funcs.
type EventFuncCtx func(context.Context, int64)

// NewScheduler returns a Scheduler populated with maps.
func NewScheduler() *Scheduler {
	return &Scheduler{
		jobs:           make(map[int64]*tickerEntry),
		tickers:        make(map[time.Duration]*Ticker),
		dormantTickers: make(map[time.Duration]*Ticker),
		alarms:         make(map[int64]*Alarm),
	}
}

//
// Tickers
// Repeating events
//

func (sc *Scheduler) addTicker(d time.Duration, f EventFunc) int64 {
	sc.Lock()
	defer sc.Unlock()
	// ensure we have a Ticker for this duration
	t, ok := sc.tickers[d]
	if !ok {
		t, ok = sc.dormantTickers[d]
		if ok {
			delete(sc.dormantTickers, d)
			sc.tickers[d] = t
		} else {
			t = NewTicker(d)
			sc.tickers[d] = t
		}
	}
	sc.id++
	id := sc.id
	// register job entry
	sc.jobs[id] = &tickerEntry{id: id, duration: d}
	// add to ticker and start (legacy EventFunc will be adapted by Ticker.AddFunc)
	t.AddFunc(f, id)
	t.Start()
	return id
}

// addTickerCtx registers a context-aware callback and returns the id.
func (sc *Scheduler) addTickerCtx(d time.Duration, f EventFuncCtx) int64 {
	sc.Lock()
	defer sc.Unlock()
	// ensure we have a Ticker for this duration
	t, ok := sc.tickers[d]
	if !ok {
		t, ok = sc.dormantTickers[d]
		if ok {
			delete(sc.dormantTickers, d)
			sc.tickers[d] = t
		} else {
			t = NewTicker(d)
			sc.tickers[d] = t
		}
	}
	sc.id++
	id := sc.id
	// register job entry
	sc.jobs[id] = &tickerEntry{id: id, duration: d}
	// add context-aware func
	t.AddFuncCtx(f, id)
	t.Start()
	return id
}

// RepeatHandle is the ergonomic variant that returns an opaque Handle.
// It accepts a context-aware callback.
func (sc *Scheduler) RepeatHandle(d time.Duration, f EventFuncCtx) Handle {
	id := sc.addTickerCtx(d, f)
	return Handle{scheduler: sc, id: id, kind: "ticker"}
}

// RepeatHandleFunc is a convenience wrapper that accepts a legacy EventFunc
// and returns an opaque Handle. It preserves backwards compatibility while
// providing ergonomic handles.
func (sc *Scheduler) RepeatHandleFunc(d time.Duration, f EventFunc) Handle {
	id := sc.addTicker(d, f)
	return Handle{scheduler: sc, id: id, kind: "ticker"}
}

// RepeatWithContext registers a context-aware repeating callback and returns
// the assigned job id. This mirrors Repeat but for EventFuncCtx.
func (sc *Scheduler) RepeatWithContext(d time.Duration, f EventFuncCtx) int64 {
	return sc.addTickerCtx(d, f)
}

// Repeat adds a repeating task based on an interval and returns a job id.
func (sc *Scheduler) Repeat(d time.Duration, f EventFunc) int64 {
	return sc.addTicker(d, f)
}

// RemoveTicker removes a ticker by duration, stopping it if necessary and
// removing any job entries that belonged to that duration.
func (sc *Scheduler) RemoveTicker(d time.Duration) {
	sc.Lock()
	defer sc.Unlock()
	t, ok := sc.tickers[d]
	if ok {
		// stop the ticker and move to dormant
		t.Stop()
		delete(sc.tickers, d)
		sc.dormantTickers[d] = t
	}
	// remove job entries for this duration
	for id, je := range sc.jobs {
		if je.duration == d {
			delete(sc.jobs, id)
		}
	}
}

// RemoveTickerByID removes a single registered repeating job by id. This
// variant removes the registration and, if the containing ticker becomes
// empty, moves it to dormantTickers and triggers a stop. It does NOT wait for
// the ticker to finish handlers; Stop is invoked asynchronously so this call
// returns promptly.
func (sc *Scheduler) RemoveTickerByID(id int64) {
	// remove from maps under lock
	sc.Lock()
	je, ok := sc.jobs[id]
	if !ok {
		sc.Unlock()
		return
	}
	d := je.duration
	// find ticker for this duration
	t, tok := sc.tickers[d]
	if tok {
		// remove func from ticker
		empty := t.RemoveFunc(id)
		// remove job entry
		delete(sc.jobs, id)
		if empty {
			// move to dormant while still holding lock
			delete(sc.tickers, d)
			sc.dormantTickers[d] = t
			// call Stop asynchronously after unlock so we don't block while
			// waiting for handler goroutines (and avoid potential lock
			// ordering deadlocks if handlers call back into the scheduler).
			sc.Unlock()
			go t.Stop()
			return
		}
	}
	sc.Unlock()
}

// RemoveAndWaitByID removes a registered repeating job by id and waits for any
// in-flight handler goroutines to complete. If the containing ticker becomes
// empty, it is stopped synchronously.
func (sc *Scheduler) RemoveAndWaitByID(id int64) {
	// remove from maps under lock
	sc.Lock()
	je, ok := sc.jobs[id]
	if !ok {
		sc.Unlock()
		return
	}
	d := je.duration
	t, tok := sc.tickers[d]
	if tok {
		// remove func from ticker
		empty := t.RemoveFunc(id)
		// remove job entry
		delete(sc.jobs, id)
		if empty {
			// move to dormant and stop synchronously after unlocking
			delete(sc.tickers, d)
			sc.dormantTickers[d] = t
			sc.Unlock()
			// Stop waits for supervisor and handlers to finish
			t.Stop()
			return
		}
	}
	sc.Unlock()
}

//
// Alarms
// One-time events
//

func (sc *Scheduler) addAlarm(d time.Duration, f EventFunc) int64 {
	sc.Lock()
	defer sc.Unlock()
	sc.id++
	alarm := NewAlarm(d, sc.id, f)
	alarm.scheduler = sc
	sc.alarms[sc.id] = alarm
	alarm.Start()
	return sc.id
}

func (sc *Scheduler) addAlarmCtx(d time.Duration, f EventFuncCtx) int64 {
	sc.Lock()
	defer sc.Unlock()
	sc.id++
	alarm := NewAlarmCtx(d, sc.id, f)
	alarm.scheduler = sc
	sc.alarms[sc.id] = alarm
	alarm.Start()
	return sc.id
}

// RemoveAlarm removes an alarm by id, stopping it if necessary.
func (sc *Scheduler) RemoveAlarm(id int64) {
	// remove from map under lock first to avoid reentrant calls
	sc.Lock()
	alarm, ok := sc.alarms[id]
	if ok {
		delete(sc.alarms, id)
	}
	sc.Unlock()
	if ok {
		// stop the alarm outside the scheduler lock
		alarm.Stop()
	}
}

// RemoveAlarmAndWait is an explicit variant that removes the alarm and waits
// for any in-flight handler goroutines to complete. This mirrors
// RemoveAlarm's behavior but is provided for API symmetry.
func (sc *Scheduler) RemoveAlarmAndWait(id int64) {
	// same behavior as RemoveAlarm
	sc.Lock()
	alarm, ok := sc.alarms[id]
	if ok {
		delete(sc.alarms, id)
	}
	sc.Unlock()
	if ok {
		alarm.Stop()
	}
}

// AddAlarmIn triggers functions after a specific duration has passed.
func (sc *Scheduler) AddAlarmIn(d time.Duration, f EventFunc) int64 {
	return sc.addAlarm(d, f)
}

// AddAlarmAt triggers functions at a specific time of day.
func (sc *Scheduler) AddAlarmAt(t time.Time, f EventFunc) int64 {
	when := time.Until(t)
	return sc.addAlarm(when, f)
}

// AddAlarmInHandle registers a context-aware alarm and returns a Handle.
func (sc *Scheduler) AddAlarmInHandle(d time.Duration, f EventFuncCtx) Handle {
	id := sc.addAlarmCtx(d, f)
	return Handle{scheduler: sc, id: id, kind: "alarm"}
}

// AddAlarmInHandleFunc registers a legacy EventFunc-based alarm and returns a
// Handle for ergonomics.
func (sc *Scheduler) AddAlarmInHandleFunc(d time.Duration, f EventFunc) Handle {
	id := sc.addAlarm(d, f)
	return Handle{scheduler: sc, id: id, kind: "alarm"}
}

// AddAlarmAtHandle registers a context-aware alarm at a specific time and
// returns a Handle.
func (sc *Scheduler) AddAlarmAtHandle(t time.Time, f EventFuncCtx) Handle {
	when := time.Until(t)
	id := sc.addAlarmCtx(when, f)
	return Handle{scheduler: sc, id: id, kind: "alarm"}
}

// AddAlarmAtHandleFunc registers a legacy EventFunc-based alarm at a specific
// time and returns a Handle.
func (sc *Scheduler) AddAlarmAtHandleFunc(t time.Time, f EventFunc) Handle {
	when := time.Until(t)
	id := sc.addAlarm(when, f)
	return Handle{scheduler: sc, id: id, kind: "alarm"}
}

// AddAlarmInWithContext triggers functions after a specific duration has
// passed and returns the job id for EventFuncCtx-style callbacks.
func (sc *Scheduler) AddAlarmInWithContext(d time.Duration, f EventFuncCtx) int64 {
	return sc.addAlarmCtx(d, f)
}

// AddAlarmAtWithContext triggers functions at a specific time and returns
// the job id for EventFuncCtx-style callbacks.
func (sc *Scheduler) AddAlarmAtWithContext(t time.Time, f EventFuncCtx) int64 {
	when := time.Until(t)
	return sc.addAlarmCtx(when, f)
}

// StopAlarms stops all alarms.
func (sc *Scheduler) StopAlarms() {
	// copy alarms under lock to avoid mutation during iteration
	sc.Lock()
	alarms := make([]*Alarm, 0, len(sc.alarms))
	for _, a := range sc.alarms {
		alarms = append(alarms, a)
	}
	// clear map
	sc.alarms = make(map[int64]*Alarm)
	sc.Unlock()
	for _, a := range alarms {
		a.Stop()
	}
}

// StopTickers stops all tickers.
func (sc *Scheduler) StopTickers() {
	sc.Lock()
	tickers := make([]*Ticker, 0, len(sc.tickers))
	for _, t := range sc.tickers {
		tickers = append(tickers, t)
	}
	// clear map while holding lock
	sc.tickers = make(map[time.Duration]*Ticker)
	// clear job entries as well
	for id := range sc.jobs {
		delete(sc.jobs, id)
	}
	sc.Unlock()
	for _, t := range tickers {
		t.Stop()
	}
}

// StopAll alarms and tickers.
func (sc *Scheduler) StopAll() {
	sc.StopAlarms()
	sc.StopTickers()
}

// Handle is an opaque handle for a registered ticker or alarm.
// It carries the scheduler pointer so methods can be invoked directly.
type Handle struct {
	scheduler *Scheduler
	id        int64
	kind      string // "ticker" or "alarm"
}

// Remove removes the underlying job (ticker or alarm) without waiting for
// in-flight handlers to finish.
func (h Handle) Remove() {
	if h.scheduler == nil {
		return
	}
	switch h.kind {
	case "ticker":
		h.scheduler.RemoveTickerByID(h.id)
	case "alarm":
		h.scheduler.RemoveAlarm(h.id)
	}
}

// RemoveAndWait removes the underlying job and waits for any in-flight
// handler goroutines to complete.
func (h Handle) RemoveAndWait() {
	if h.scheduler == nil {
		return
	}
	switch h.kind {
	case "ticker":
		h.scheduler.RemoveAndWaitByID(h.id)
	case "alarm":
		h.scheduler.RemoveAlarmAndWait(h.id)
	}
}

// Wait for all waitgroups in tickers and alarms.
func (sc *Scheduler) Wait() {
	// snapshot tickers and alarms under read lock
	sc.RLock()
	tickers := make([]*Ticker, 0, len(sc.tickers))
	for _, t := range sc.tickers {
		tickers = append(tickers, t)
	}
	alarms := make([]*Alarm, 0, len(sc.alarms))
	for _, a := range sc.alarms {
		alarms = append(alarms, a)
	}
	sc.RUnlock()
	for _, t := range tickers {
		t.Wait()
	}
	for _, a := range alarms {
		a.Wait()
	}
}
