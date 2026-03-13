package gon

import (
	"sync"
	"time"
)

// Scheduler holds pointers to all the tickers and timers.
type Scheduler struct {
	sync.RWMutex
	id             int64
	tickers        map[time.Duration]*Ticker
	dormantTickers map[time.Duration]*Ticker
	alarms         map[int64]*Alarm
}

// EventFunc is the signature of the ticker/alarm callbacks.
type EventFunc func(int64)

// NewScheduler returns a Scheduler populated with maps.
func NewScheduler() *Scheduler {
	return &Scheduler{
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
	t.AddFunc(f, sc.id)
	t.Start()
	return sc.id
}

// RemoveTicker removes a ticker by duration, stopping it if necessary.
func (sc *Scheduler) RemoveTicker(d time.Duration) {
	sc.Lock()
	defer sc.Unlock()
	t, ok := sc.tickers[d]
	if ok {
		t.Stop()
		delete(sc.tickers, d)
		sc.dormantTickers[d] = t
	}
}

// RepeatS adds a repeating task based on an interval.
func (sc *Scheduler) Repeat(d time.Duration, f EventFunc) int64 {
	return sc.addTicker(d, f)
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

// AddAlarmIn triggers functions after a specific duration has passed.
func (sc *Scheduler) AddAlarmIn(d time.Duration, f EventFunc) int64 {
	return sc.addAlarm(d, f)
}

// AddAlarmAt triggers functions at a specific time of day.
func (sc *Scheduler) AddAlarmAt(t time.Time, f EventFunc) int64 {
	when := time.Until(t)
	return sc.addAlarm(when, f)
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
