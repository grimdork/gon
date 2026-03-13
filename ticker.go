package gon

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Ticker runs one or more functions, repeating at an interval.
type Ticker struct {
	sync.RWMutex
	sync.WaitGroup
	duration time.Duration
	ticker   *time.Ticker
	// funcs holds context-aware callbacks keyed by id.
	funcs map[int64]EventFuncCtx
	// handlers holds cancel funcs for in-flight handler goroutines.
	handlers map[int64]context.CancelFunc
	// handlerTokens tracks a generation token per job id so finishing
	// handlers don't remove a newer cancel func.
	handlerTokens map[int64]uint64
	// tokenCounter increments to produce unique tokens.
	tokenCounter uint64
	quit         chan struct{}
	running      bool
}

// NewTicker creates the Ticker structure and quit channel.
func NewTicker(d time.Duration) *Ticker {
	t := &Ticker{
		duration:      d,
		funcs:         make(map[int64]EventFuncCtx),
		handlers:      make(map[int64]context.CancelFunc),
		handlerTokens: make(map[int64]uint64),
		quit:          make(chan struct{}, 1),
	}
	return t
}

// AddFunc adds another callback to the funcs map with a new ID.
// This accepts the old signature and adapts it to a context-aware callback.
func (t *Ticker) AddFunc(f EventFunc, id int64) {
	t.AddFuncCtx(func(ctx context.Context, id int64) {
		// Adapter: ignore context and call old-style func
		f(id)
	}, id)
}

// AddFuncCtx adds a context-aware callback directly.
func (t *Ticker) AddFuncCtx(f EventFuncCtx, id int64) {
	t.Lock()
	defer t.Unlock()
	t.funcs[id] = f
}

// RemoveFunc removes a callback by id. It returns true if the ticker
// has no more funcs registered after removal. It also cancels any
// in-flight handler goroutines for that id (non-blocking).
func (t *Ticker) RemoveFunc(id int64) bool {
	t.Lock()
	defer t.Unlock()
	delete(t.funcs, id)
	if cancel, ok := t.handlers[id]; ok {
		// cancel running handler(s)
		cancel()
		delete(t.handlers, id)
	}
	return len(t.funcs) == 0
}

// Start creates the time.Ticker and handles the calls at intervals.
func (t *Ticker) Start() {
	if t.running {
		return
	}

	t.Lock()
	if t.running {
		t.Unlock()
		return
	}
	t.running = true
	t.ticker = time.NewTicker(t.duration)
	// mark supervisor goroutine
	t.Add(1)
	go func() {
		defer t.Add(-1)
		for {
			select {
			case <-t.ticker.C:
				// snapshot funcs under read lock to avoid holding lock while calling
				t.RLock()
				pairs := make([]struct {
					id int64
					f  EventFuncCtx
				}, 0, len(t.funcs))
				for id, f := range t.funcs {
					pairs = append(pairs, struct {
						id int64
						f  EventFuncCtx
					}{id: id, f: f})
				}
				t.RUnlock()
				for _, p := range pairs {
					// create cancellable context for each handler so it can be
					// cancelled by Remove or Stop.
					ctx, cancel := context.WithCancel(context.Background())
					// generate token for this handler generation
					tok := atomic.AddUint64(&t.tokenCounter, 1)
					t.Lock()
					// store/replace cancel and token for this id
					t.handlers[p.id] = cancel
					t.handlerTokens[p.id] = tok
					t.Unlock()
					t.Add(1)
					go func(pid int64, tf EventFuncCtx, token uint64) {
						defer t.Done()
						// run handler
						tf(ctx, pid)
						// cleanup cancel entry after handler finishes only if the
						// token matches (prevents removing a newer generation).
						t.Lock()
						if t.handlerTokens[pid] == token {
							delete(t.handlerTokens, pid)
							delete(t.handlers, pid)
						}
						t.Unlock()
					}(p.id, p.f, tok)
				}
			case <-t.quit:
				t.ticker.Stop()
				// cancel any running handlers
				t.Lock()
				for _, c := range t.handlers {
					c()
				}
				// clear handlers and funcs map
				for k := range t.handlers {
					delete(t.handlers, k)
				}
				for k := range t.funcs {
					delete(t.funcs, k)
				}
				t.running = false
				t.Unlock()
				return
			}
		}
	}()
	t.Unlock()
}

// Stop the ticker.
func (t *Ticker) Stop() {
	// signal quit non-blocking
	select {
	case t.quit <- struct{}{}:
	default:
	}
	// wait for supervisor and handlers
	t.Wait()
}
