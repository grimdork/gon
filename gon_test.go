package gon_test

import (
	"testing"
	"time"

	"github.com/grimdork/gon"
)

func TestAlarm(t *testing.T) {
	sc := gon.NewScheduler()
	when := time.Now().Add(time.Second * 10)
	event := func(id int64) {
		t.Logf("Alarm with id %d fired\n", id)
	}
	id := sc.AddAlarmAt(when, event, false)
	t.Logf("Main thread: Created alarm with id %d\n", id)

	tick := func(id int64) {
		t.Log("Tick.")
	}
	tick3 := func(id int64) {
		t.Logf("3-second ticker with id %d fired\n", id)
	}
	tick5 := func(id int64) {
		t.Logf("5-second ticker with id %d fired\n", id)
	}
	tick10 := func(id int64) {
		t.Logf("10-second ticker with id %d fired\n", id)
	}

	id = sc.Repeat(time.Second, tick)
	t.Logf("Main thread: added 1-second with id %d\n", id)
	id = sc.Repeat(3*time.Second, tick3)
	t.Logf("Main thread: added 3-second ticker with id %d", id)
	id = sc.Repeat(5*time.Second, tick5)
	t.Logf("Main thread: added 5-second ticker with id %d", id)
	id = sc.Repeat(10*time.Second, tick10)
	t.Logf("Main thread: added 10-second ticker with id %d", id)

	time.Sleep(time.Second * 11)
	sc.Wait()
}
