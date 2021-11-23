# gon
Fire and forget alarm/ticker scheduling.

## What
This package creates a scheduler which creates one-shot alarms and repeating tickers which call a function.

## How
Add an alarm:
```go
	sc := gon.NewScheduler()
	id = sc.AddAlarmAt(time.Now().Add(time.Second*10), func(id int64) {
		fmt.Printf("Alarm with id %d fired\n", id)
	})
	fmt.Printf("Alarm %d added\n", id)
```

Add a repeating function:
```go
	sc := gon.NewScheduler()
	id = sc.Repeat(time.Second*3, func(i int64) {
		fmt.Printf("3-second ticker with id %d fired\n", i)
	})
	fmt.Printf("Added 3-second ticker with id %d\n", id)
```

The IDs for alarms and tickers are all unique.
