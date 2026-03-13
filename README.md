# gon
Fire-and-forget alarm and ticker scheduling — small, concurrency-safe helper for Go.

This README prefers the modern, ergonomic API first (Handle + context-aware callbacks), then shows the legacy API and migration notes.

Recommended (modern) API

The modern API returns opaque Handles and supports context-aware callbacks. Use these in new code.

Example: repeating task with a Handle and context-aware callback

```go
sc := gon.NewScheduler()
// Context-aware handler signature: func(ctx context.Context, id int64)
h := sc.RepeatHandle(3*time.Second, func(ctx context.Context, id int64) {
    // respect ctx.Done() and clean up promptly when cancelled
    select {
    case <-ctx.Done():
        return
    case <-time.After(100 * time.Millisecond):
        fmt.Printf("tick %d\n", id)
    }
})
// later: remove without waiting
h.Remove()
// or remove and wait for any in-flight handler to finish
h.RemoveAndWait()
```

Example: one-shot alarm (Handle + context-aware)

```go
h := sc.AddAlarmInHandle(10*time.Second, func(ctx context.Context, id int64) {
    fmt.Printf("alarm %d fired\n", id)
})
// cancel before it fires
h.RemoveAndWait()
```

Batch removal (handles)

```go
// collect handles when creating jobs
handles := make([]gon.Handle, 0, 10)
for i := 0; i < 10; i++ {
    handles = append(handles, sc.RepeatHandle(time.Second, func(ctx context.Context, id int64) {
        // work
    }))
}
// remove all and wait for clean shutdown
for _, h := range handles {
    h.RemoveAndWait()
}
```

StopAll (stop everything, wait)

```go
// StopAll stops all alarms and tickers and waits for handlers to finish
sc.StopAll()
// sc.Wait() is useful if you used non-blocking removals earlier and want
// to wait for the remaining live jobs to finish.
sc.Wait()
```

Graceful server shutdown example

```go
// typical pattern for shutting down services that use gon for background tasks
func runServer() error {
    sc := gon.NewScheduler()
    // start server and background jobs...

    // wait for termination signal (context cancellation etc.)
    <-ctx.Done()

    // cancel and wait for clean shutdown of scheduled work
    sc.StopAll()
    // optionally wait for any remaining items in legacy maps
    sc.Wait()
    return nil
}
```

Context-aware variants that return numeric ids

If you want numeric ids instead of Handles, the WithContext helpers return the id and accept EventFuncCtx:

- RepeatWithContext(d time.Duration, f EventFuncCtx) int64
- AddAlarmInWithContext(d time.Duration, f EventFuncCtx) int64

Legacy API (backwards-compatible)

The legacy API is preserved and works as before. Use it when migrating incrementally.

Example: legacy repeating task

```go
id := sc.Repeat(3*time.Second, func(id int64) {
    fmt.Printf("legacy tick %d\n", id)
})
// remove by id - ergonomic RemoveTickerByID exists too
sc.RemoveTickerByID(id)
```

Example: legacy alarm

```go
id := sc.AddAlarmIn(5*time.Second, func(id int64) {
    fmt.Printf("legacy alarm %d\n", id)
})
sc.RemoveAlarm(id)
```

Advanced: combining legacy and modern handlers

```go
// create some legacy jobs and some context-aware jobs
id := sc.Repeat(500*time.Millisecond, func(id int64) { /* legacy */ })
h := sc.RepeatHandle(500*time.Millisecond, func(ctx context.Context, id int64) { /* ctx-aware */ })

// remove legacy by id and handle by Handle
sc.RemoveTickerByID(id)
h.RemoveAndWait()
```

When to prefer the modern API

- Prefer Handles when you need ergonomic per-job control (Stop/Remove/Wait) or plan to attach behavior to the job handle later.
- Prefer EventFuncCtx when handlers may block or need cleanup: the scheduler cancels the provided context when the job is removed or the scheduler stops.

Migration notes

- RepeatHandleFunc / AddAlarmInHandleFunc accept legacy EventFunc and return a Handle for ergonomic control while keeping the old callback signature.
- RemoveTicker(d) and other duration-grouped operations are still available for backwards compatibility, but per-job removal via handles or RemoveTickerByID is recommended.

Testing and CI

- Fast mode for tests: set GON_FAST_TEST=1 to run timing tests in a short window (default fast window is 300ms).
- Custom test window: set GON_TEST_WINDOW_MS to a number of milliseconds.
- Recommended CI command: GON_FAST_TEST=1 go test -race ./...

Concurrency guarantees

- Handlers run with a cancellable context; the scheduler cancels the context on Remove/Stop to allow graceful handler termination.
- Stop/Remove operations avoid holding the scheduler lock while performing blocking Stop() calls to prevent deadlocks.
- Stop/Wait and RemoveAndWait semantics ensure WaitGroups are used so callers can deterministically wait for goroutine shutdown.

Compatibility & stability

- All legacy APIs remain available and continue to work. The new APIs are additive.
- The package aims to remain small and non-invasive; default behaviors are unchanged unless you opt into the modern helpers.

Examples and snippets

- See gon_test.go and gon_handle_test.go for more examples and tests demonstrating Handle behavior and the context-aware callbacks.

Contributing

- Run `go test -race ./...` locally and in CI.
- Keep changes small and well-tested; prefer adding tests for any concurrency changes.

License

- MIT (same as the project repository).
