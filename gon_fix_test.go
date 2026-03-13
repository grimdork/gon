package gon

import (
	"testing"
	"time"
)

func TestAlarmRemoveNoRecursion(t *testing.T) {
	sc := NewScheduler()
	fired := make(chan struct{})
	id := sc.AddAlarmIn(time.Millisecond*50, func(i int64) {
		close(fired)
	})
	// Wait for it to fire
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("alarm did not fire")
	}
	// Ensure alarm removed
	sc.Wait()
	// After firing, removing should be a no-op and not panic
	sc.RemoveAlarm(id)
}

func TestTickerStopWaits(t *testing.T) {
	sc := NewScheduler()
	ch := make(chan struct{})
	id := sc.Repeat(time.Millisecond*20, func(i int64) {
		ch <- struct{}{}
	})
	// expect a few ticks
	for i := 0; i < 3; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("expected tick %d", i)
		}
	}
	// stop ticker by removing its duration
	sc.RemoveTicker(20 * time.Millisecond)
	// ensure no panic and wait returns
	sc.Wait()
	_ = id
}
