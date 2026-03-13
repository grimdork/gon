package gon

import (
	"context"
	"testing"
	"time"
)

func TestRemoveAndWaitByIDTicker(t *testing.T) {
	sc := NewScheduler()
	started := make(chan struct{})
	finished := make(chan struct{})
	id := sc.RepeatWithContext(20*time.Millisecond, func(ctx context.Context, _ int64) {
		// signal started once
		select {
		case <-started:
		default:
			close(started)
		}
		// block until cancelled
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
		close(finished)
	})
	// wait for handler to start
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	// remove and wait; should cancel handler and wait for finished
	sc.RemoveAndWaitByID(id)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish after RemoveAndWaitByID")
	}
}

func TestRemoveAlarmAndWait(t *testing.T) {
	sc := NewScheduler()
	fired := make(chan struct{})
	// schedule far in the future
	id := sc.AddAlarmInWithContext(time.Second*5, func(ctx context.Context, _ int64) {
		close(fired)
	})
	// remove and wait immediately
	sc.RemoveAlarmAndWait(id)
	// ensure it did not fire
	select {
	case <-fired:
		t.Fatal("alarm fired despite RemoveAlarmAndWait")
	case <-time.After(50 * time.Millisecond):
		// good
	}
}
