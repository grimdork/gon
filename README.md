# gon

[![Go tests](https://github.com/grimdork/gon/actions/workflows/go.yml/badge.svg)](https://github.com/grimdork/gon/actions/workflows/go.yml)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/grimdork/gon)](https://pkg.go.dev/github.com/grimdork/gon)
[![Go Report Card](https://goreportcard.com/badge/github.com/grimdork/gon)](https://goreportcard.com/report/github.com/grimdork/gon)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Fire-and-forget alarm and ticker scheduling — small, concurrency-safe helper for Go.

This README shows the modern ergonomic API first (Handle + context-aware callbacks), then legacy usage, testing tips, and CI notes.

Installation

Get the package:

```sh
go get github.com/grimdork/gon@latest
```

Quick overview

- Modern: opaque Handle + context-aware callbacks (preferred).
- Legacy: numeric IDs + simple callbacks (kept for compatibility).
- Testing: set GON_FAST_TEST=1 to run timing tests quickly in CI or locally.

Recommended (modern) API

Example: repeating task with a Handle and context-aware callback

```go
sc := gon.NewScheduler()
// Context-aware handler signature: func(ctx context.Context, id int64)
h := sc.RepeatHandle(3*time.Second, func(ctx context.Context, id int64) {
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

One-shot alarm (Handle + context-aware)

```go
h := sc.AddAlarmInHandle(10*time.Second, func(ctx context.Context, id int64) {
    fmt.Printf("alarm %d fired\n", id)
})
// cancel before it fires
h.RemoveAndWait()
```

API quick reference (modern)

- type Handle: opaque handle with methods Remove() and RemoveAndWait()
- RepeatHandle(d time.Duration, f EventFuncCtx) Handle
- AddAlarmInHandle(d time.Duration, f EventFuncCtx) Handle
- RepeatWithContext(d time.Duration, f EventFuncCtx) int64
- AddAlarmInWithContext(d time.Duration, f EventFuncCtx) int64
- EventFuncCtx signature: func(ctx context.Context, id int64)

Legacy API (backwards-compatible)

The legacy API remains available. Example:

```go
id := sc.Repeat(3*time.Second, func(id int64) {
    fmt.Printf("legacy tick %d\n", id)
})
sc.RemoveTickerByID(id)
```

Advanced usage

Batch removal (handles):

```go
handles := make([]gon.Handle, 0, 10)
for i := 0; i < 10; i++ {
    handles = append(handles, sc.RepeatHandle(time.Second, func(ctx context.Context, id int64) {}))
}
for _, h := range handles {
    h.RemoveAndWait()
}
```

StopAll (stop everything, wait)

```go
sc.StopAll()
sc.Wait()
```

Graceful server shutdown pattern

```go
// on termination:
sc.StopAll()
sc.Wait()
```

Testing and CI

- Fast mode for tests: set GON_FAST_TEST=1. This shortens the timing windows used in tests (useful for CI).
- Custom test window: set GON_TEST_WINDOW_MS to control the test timing window (milliseconds).
- Recommended CI command: `GON_FAST_TEST=1 go test -race ./...`

CI notes

- Prefer running tests in the official golang container in CI to avoid host toolchain cache issues. Example workflow uses `container: golang:1.25` to ensure a clean toolchain.
- Avoid caching compiled build artifacts across different Go versions; cache only module downloads if needed.

Migration hints

- Add Handle-returning variants (RepeatHandle/AddAlarmInHandle) where ergonomic control is helpful.
- Use EventFuncCtx when handlers may block or need cancellation: the scheduler cancels the provided context when the job is removed or the scheduler stops.

Contributing

- Run `GON_FAST_TEST=1 go test -race ./...` locally. Keep changes small and include tests for concurrency changes.

License

- MIT
