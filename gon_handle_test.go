package gon

import (
	"context"
	"testing"
	"time"
)

func TestHandleTickerRemoveAndWait(t *testing.T) {
	sc := NewScheduler()
	started := make(chan struct{})
	finished := make(chan struct{})
	h := sc.RepeatHandle(20*time.Millisecond, func(ctx context.Context, id int64) {
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
	h.RemoveAndWait()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish after RemoveAndWait")
	}
}

func TestHandleAlarmCancelBeforeFire(t *testing.T) {
	sc := NewScheduler()
	fired := make(chan struct{})
	// schedule far in the future
	h := sc.AddAlarmInHandle(time.Second*5, func(ctx context.Context, id int64) {
		close(fired)
	})
	// remove and wait immediately
	h.RemoveAndWait()
	// ensure it did not fire
	select {
	case <-fired:
		t.Fatal("alarm fired despite RemoveAndWait")
	case <-time.After(50 * time.Millisecond):
		// good
	}
}

func TestLegacyRepeatStillWorks(t *testing.T) {
	sc := NewScheduler()
	ch := make(chan struct{})
	id := sc.Repeat(10*time.Millisecond, func(i int64) { ch <- struct{}{} })
	// expect a tick
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("expected tick")
	}
	// cleanup
	sc.RemoveTickerByID(id)
	sc.Wait()
}
