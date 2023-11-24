package runnable

import (
	"context"
	"errors"
	"sync"
)

var (
	errRunnableGroupStopped = errors.New("can't accept new runnable as stop procedure is already engaged")
)

// Runnable allows a component to be started.
// It's very important that Start blocks until
// it's done running.
type Runnable interface {
	// Start starts running the component.  The component will stop running
	// when the context is closed. Start blocks until the context is closed or
	// an error occurs.
	Start(context.Context) error
}

// readyRunnable encapsulates a runnable with
// a ready check.
type readyRunnable struct {
	Runnable
	Check       runnableCheck
	signalReady bool
}

// runnableCheck can be passed to Add() to let the runnable group determine that a
// runnable is ready. A runnable check should block until a runnable is ready,
// if the returned result is false, the runnable is considered not ready and failed.
type runnableCheck func(ctx context.Context) bool

// runnableGroup manages a group of runnables that are
// meant to be running together until StopAndWait is called.
//
// Runnables can be added to a group after the group has started
// but not after it's stopped or while shutting down.
type RunnableGroup struct {
	ctx    context.Context
	cancel context.CancelFunc

	start        sync.Mutex
	startOnce    sync.Once
	started      bool
	startQueue   []*readyRunnable
	startReadyCh chan *readyRunnable

	stop     sync.RWMutex
	stopOnce sync.Once
	stopped  bool

	// errChan is the error channel passed by the caller
	// when the group is created.
	// All errors are forwarded to this channel once they occur.
	errChan chan error

	// ch is the internal channel where the runnables are read off from.
	ch chan *readyRunnable

	// wg is an internal sync.WaitGroup that allows us to properly stop
	// and wait for all the runnables to finish before returning.
	wg *sync.WaitGroup
}

func NewRunnableGroup(errChan chan error) *RunnableGroup {
	r := &RunnableGroup{
		startReadyCh: make(chan *readyRunnable),
		errChan:      errChan,
		ch:           make(chan *readyRunnable),
		wg:           new(sync.WaitGroup),
	}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	return r
}

// Started returns true if the group has started.
func (r *RunnableGroup) Started() bool {
	r.start.Lock()
	defer r.start.Unlock()
	return r.started
}

// Start starts the group and waits for all
// initially registered runnables to start.
// It can only be called once, subsequent calls have no effect.
func (r *RunnableGroup) Start(ctx context.Context) error {
	var retErr error

	r.startOnce.Do(func() {
		defer close(r.startReadyCh)

		// Start the internal reconciler.
		go r.reconcile()

		// Start the group and queue up all
		// the runnables that were added prior.
		r.start.Lock()
		r.started = true
		for _, rn := range r.startQueue {
			rn.signalReady = true
			r.ch <- rn
		}
		r.start.Unlock()

		// If we don't have any queue, return.
		if len(r.startQueue) == 0 {
			return
		}

		// Wait for all runnables to signal.
		for {
			select {
			case <-ctx.Done():
				if err := ctx.Err(); !errors.Is(err, context.Canceled) {
					retErr = err
				}
			case rn := <-r.startReadyCh:
				for i, existing := range r.startQueue {
					if existing == rn {
						// Remove the item from the start queue.
						r.startQueue = append(r.startQueue[:i], r.startQueue[i+1:]...)
						break
					}
				}
				// We're done waiting if the queue is empty, return.
				if len(r.startQueue) == 0 {
					return
				}
			}
		}
	})

	return retErr
}

// reconcile is our main entrypoint for every runnable added
// to this group. Its primary job is to read off the internal channel
// and schedule runnables while tracking their state.
func (r *RunnableGroup) reconcile() {
	for runnable := range r.ch {
		// Handle stop.
		// If the shutdown has been called we want to avoid
		// adding new goroutines to the WaitGroup because Wait()
		// panics if Add() is called after it.
		{
			r.stop.RLock()
			if r.stopped {
				// Drop any runnables if we're stopped.
				r.errChan <- errRunnableGroupStopped
				r.stop.RUnlock()
				continue
			}

			// Why is this here?
			// When StopAndWait is called, if a runnable is in the process
			// of being added, we could end up in a situation where
			// the WaitGroup is incremented while StopAndWait has called Wait(),
			// which would result in a panic.
			r.wg.Add(1)
			r.stop.RUnlock()
		}

		// Start the runnable.
		go func(rn *readyRunnable) {
			go func() {
				if rn.Check(r.ctx) {
					if rn.signalReady {
						r.startReadyCh <- rn
					}
				}
			}()

			// If we return, the runnable ended cleanly
			// or returned an error to the channel.
			//
			// We should always decrement the WaitGroup here.
			defer r.wg.Done()

			// Start the runnable.
			if err := rn.Start(r.ctx); err != nil {
				r.errChan <- err
			}
		}(runnable)
	}
}

// Add Runnable without runnableCheck
func (r *RunnableGroup) AddDefault(rn Runnable) error {
	return r.Add(rn, nil)
}

// Add should be able to be called before and after Start, but not after StopAndWait.
// Add should return an error when called during StopAndWait.
func (r *RunnableGroup) Add(rn Runnable, ready runnableCheck) error {
	r.stop.RLock()
	if r.stopped {
		r.stop.RUnlock()
		return errRunnableGroupStopped
	}
	r.stop.RUnlock()

	if ready == nil {
		ready = func(_ context.Context) bool { return true }
	}

	readyRunnable := &readyRunnable{
		Runnable: rn,
		Check:    ready,
	}

	// Handle start.
	// If the overall runnable group isn't started yet
	// we want to buffer the runnables and let Start()
	// queue them up again later.
	{
		r.start.Lock()

		// Check if we're already started.
		if !r.started {
			// Store the runnable in the internal if not.
			r.startQueue = append(r.startQueue, readyRunnable)
			r.start.Unlock()
			return nil
		}
		r.start.Unlock()
	}

	// Enqueue the runnable.
	r.ch <- readyRunnable
	return nil
}

// StopAndWait waits for all the runnables to finish before returning.
func (r *RunnableGroup) StopAndWait(ctx context.Context) {
	r.stopOnce.Do(func() {
		// Close the reconciler channel once we're done.
		defer close(r.ch)

		_ = r.Start(ctx)
		r.stop.Lock()
		// Store the stopped variable so we don't accept any new
		// runnables for the time being.
		r.stopped = true
		r.stop.Unlock()

		// Cancel the internal channel.
		r.cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			// Wait for all the runnables to finish.
			r.wg.Wait()
		}()

		select {
		case <-done:
			// We're done, exit.
		case <-ctx.Done():
			// Calling context has expired, exit.
		}
	})
}
