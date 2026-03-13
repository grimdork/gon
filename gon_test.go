package gon_test

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grimdork/gon"
)

// testWindow controls the overall test duration. Default is 30s for the
// long-running test. Fast CI/local runs can set GON_FAST_TEST=1 to use a
// small window (300ms) or set GON_TEST_WINDOW_MS explicitly.
var testWindow = 30 * time.Second

func TestMain(m *testing.M) {
	// allow fast mode via environment
	if os.Getenv("GON_FAST_TEST") == "1" {
		testWindow = 300 * time.Millisecond
	}
	if v := os.Getenv("GON_TEST_WINDOW_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			testWindow = time.Duration(ms) * time.Millisecond
		}
	}
	// safety cap: ensure the whole suite can't run forever. Make it relative to
	// the configured testWindow so we don't abort valid long-running tests.
	go func() {
		safeSleep := testWindow + (testWindow / 10) + 5*time.Second
		// Always allow at least 30s as a floor for safety.
		if safeSleep < 30*time.Second {
			safeSleep = 30 * time.Second
		}
		time.Sleep(safeSleep)
		fmt.Fprintln(os.Stderr, "tests exceeded safety timeout, aborting")
		os.Exit(2)
	}()
	os.Exit(m.Run())
}

func TestAlarm(t *testing.T) {
	sc := gon.NewScheduler()
	when := time.Now().Add(testWindow)

	// Counters for each ticker
	var c1, c3, c5, c10 int64

	// Alarm will fire at the end of the window and signal done
	done := make(chan struct{})
	alarmFn := func(id int64) {
		atomic.AddInt64(&c10, 1) // the 1x ticker also counts the alarm firing moment
		close(done)
	}
	id := sc.AddAlarmAt(when, alarmFn)
	t.Logf("Main thread: Created alarm with id %d (window %v)\n", id, testWindow)

	// derive ticker durations as fractions of the window
	d1 := testWindow / 10
	if d1 <= 0 {
		d1 = 10 * time.Millisecond
	}
	d3 := testWindow * 3 / 10
	d5 := testWindow * 5 / 10
	d10 := testWindow

	// Handlers increment counters
	sc.Repeat(d1, func(i int64) { atomic.AddInt64(&c1, 1) })
	sc.Repeat(d3, func(i int64) { atomic.AddInt64(&c3, 1) })
	sc.Repeat(d5, func(i int64) { atomic.AddInt64(&c5, 1) })
	sc.Repeat(d10, func(i int64) { atomic.AddInt64(&c10, 1) })

	// Wait for alarm (end of window) or timeout slightly past the window.
	select {
	case <-done:
		// normal
	case <-time.After(testWindow + (testWindow/10) + time.Second):
		t.Fatal("test window timed out")
	}

	// Stop tickers/alarms and wait for clean shutdown. RemoveTicker accepts
	// durations for backwards-compatible removal.
	sc.RemoveTicker(d1)
	sc.RemoveTicker(d3)
	sc.RemoveTicker(d5)
	sc.RemoveTicker(d10)
	// alarm already fired; ensure wait returns
	sc.Wait()

	// Validate counts: expected is approximately floor(window/interval).
	check := func(name string, got int64, interval time.Duration) {
		expected := int64(testWindow / interval)
		// allow small tolerance of 1 (timer scheduling may vary)
		if got < expected-1 || got > expected+1 {
			t.Fatalf("%s: got %d, expected ~%d (interval %v)", name, got, expected, interval)
		}
	}

	check("1x", atomic.LoadInt64(&c10), d10)
	check("1/10", atomic.LoadInt64(&c1), d1)
	check("3/10", atomic.LoadInt64(&c3), d3)
	check("5/10", atomic.LoadInt64(&c5), d5)
}
