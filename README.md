# gon
Fire-and-forget alarm and ticker scheduling (small, concurrency-safe helper).

## Overview
gon provides a lightweight Scheduler for:
- One-shot alarms (AddAlarmIn / AddAlarmAt)
- Repeating tickers (Repeat)

Callbacks use the signature: func(id int64) — the id is the scheduler-assigned handle for the alarm/ticker.

Design goals:
- Simple API for scheduling one-shot and repeating work
- Safe concurrent Stop/Remove without deadlocks or panics
- Predictable lifecycle: Stop/Remove returns once goroutines have finished

## Usage
Create a scheduler and add an alarm:

```go
sc := gon.NewScheduler()
// alarm fires once at the given time
id := sc.AddAlarmAt(time.Now().Add(10*time.Second), func(id int64) {
    fmt.Printf("Alarm %d fired\n", id)
})
fmt.Printf("Alarm %d scheduled\n", id)
```

Add a repeating task:

```go
sc := gon.NewScheduler()
// repeats every 3 seconds
id := sc.Repeat(3*time.Second, func(id int64) {
    fmt.Printf("Ticker %d fired\n", id)
})
fmt.Printf("Ticker %d added\n", id)
```

Stop and remove examples:
- RemoveAlarm(id) removes the alarm from the scheduler and stops it; callers can safely call RemoveAlarm from inside callbacks.
- RemoveTicker(d) removes the ticker for a duration d and stops it (the scheduler groups tickers by duration).
- StopAll() stops all alarms and tickers and waits for all handlers to complete.

## Concurrency guarantees
- RemoveAlarm removes the alarm under lock, then stops it outside the lock to avoid reentrant calls into the scheduler.
- Stop/Remove calls wait for any in-flight handler goroutines to finish (via WaitGroups) before returning.
- The package uses non-blocking signal sends and buffered internal quit channels to avoid goroutine leaks on races.

## Recommended CI
- Run `go test -race ./...` on CI to catch concurrency regressions.
- Lint with `golangci-lint`.

I added a GitHub Actions workflow (.github/workflows/go.yml) that runs tests with the race detector and lints on pushes and PRs to main.

## Handles and context-aware callbacks (implemented)

The scheduler supports both legacy and newer ergonomic APIs:

- Legacy callbacks: func(id int64) — preserved for backwards compatibility.
- Context-aware callbacks: func(ctx context.Context, id int64) — the scheduler provides a cancellable context to handlers which is cancelled when the job is removed or the scheduler stops.

Handle-returning APIs

- Repeat(d time.Duration, f EventFunc) int64
  - Legacy: returns a numeric id as before.
- RepeatWithContext(d time.Duration, f EventFuncCtx) int64
  - Context-aware variant that returns the numeric id.
- RepeatHandle(d time.Duration, f EventFuncCtx) Handle
  - Ergonomic variant that returns an opaque Handle for context-aware functions.
- RepeatHandleFunc(d time.Duration, f EventFunc) Handle
  - Ergonomic variant that returns an opaque Handle for legacy func(id int64).

- AddAlarmIn(d time.Duration, f EventFunc) int64
  - Legacy alarm (one-shot) returning numeric id.
- AddAlarmInWithContext(d time.Duration, f EventFuncCtx) int64
  - Context-aware alarm returning numeric id.
- AddAlarmInHandle(d time.Duration, f EventFuncCtx) Handle
  - Ergonomic alarm handle for context-aware callbacks.
- AddAlarmInHandleFunc(d time.Duration, f EventFunc) Handle
  - Ergonomic alarm handle for legacy callbacks.

Handle methods

A Handle is an opaque struct with a few helpers:

- h.Remove() — remove the job (non-blocking stop; returns promptly).
- h.RemoveAndWait() — remove the job and wait for any in-flight handlers to finish.

Cancellation

- Each handler invocation receives a context.Context which the scheduler cancels when the job is removed or when Stop/StopAll is called. Long-running handlers should observe ctx.Done() and return promptly.

Testing helpers

- Fast mode for tests: set GON_FAST_TEST=1 to run timing tests in a short window (default fast window is 300ms).
- You can also set GON_TEST_WINDOW_MS to a custom window in milliseconds for finer control during testing.

Compatibility notes

- All legacy APIs remain available and continue to work. The new context-aware APIs are additive.
- RemoveTicker(d) and other duration-based removal functions still exist for backward compatibility; new RemoveTickerByID/Handle-based removal is available for per-job control.

Recommended CI

- Run `GON_FAST_TEST=1 go test -race ./...` in CI for fast and race-enabled coverage.
- For thorough local testing, run `go test ./...` (the default TestAlarm uses a 30s window).

If you'd like, I can also add example snippets showing Handle usage and context cancellation.
