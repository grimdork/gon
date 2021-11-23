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
	sc.Lock()
	defer sc.Unlock()
	alarm, ok := sc.alarms[id]
	if ok {
		alarm.Stop()
		delete(sc.alarms, id)
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
	sc.Lock()
	defer sc.Unlock()
	for _, a := range sc.alarms {
		a.Stop()
	}
}

// StopTickers stops all tickers.
func (sc *Scheduler) StopTickers() {
	sc.Lock()
	defer sc.Unlock()
	for _, t := range sc.tickers {
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
	for _, t := range sc.tickers {
		t.Wait()
	}
	for _, a := range sc.alarms {
		a.Wait()
	}
}
