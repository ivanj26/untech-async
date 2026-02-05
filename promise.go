package untech_async

import (
	"context"
	"fmt"
	"sync/atomic"
)

// PromiseState represents the current state of a Promise (Pending, Fulfilled, or Rejected).
type PromiseState int

const (
	Pending PromiseState = iota
	Fulfilled
	Rejected
	Cancelled
)

// ThenCallback is a callback function used to process the result of a Promise.
type ThenCallback[T any] func(T) (T, error)

// CatchCallback is a callback function to catch when there is an error during processing async task in the Promise.
type CatchCallback[T any] func(context.Context, error) (T, error)

// ResolveCallback is a callback function used to process the result of a Promise.
type ResolveCallback[T any] func(T)

// RejectCallback is a callback function used to process the error of a Promise.
type RejectCallback func(error)

// Promise is a handle for an asynchronous task that will eventually return a value of type T.
//
// It allows you to execute code concurrently and retrieve the result or error when it is ready.
// where T is a type of the successful result.
type Promise[T any] struct {
	ctx context.Context

	value atomic.Value
	state atomic.Value

	settled uint32 // indicate where the promise is currently settled or not

	err error
	ch  chan struct{}
}

// NewPromise creates a new Promise that executes the given function asynchronously.
// The function receives resolve and reject callbacks to settle the promise.
func NewPromise[T any](ctx context.Context, fn func(resolveFn ResolveCallback[T], rejectFn RejectCallback)) *Promise[T] {
	if fn == nil {
		panic("fn cannot be nil/empty!")
	}

	p := &Promise[T]{
		ctx: ctx,
		ch:  make(chan struct{}),
	}
	p.state.Store(Pending)

	go func() {
		defer p.handlePanic()
		fn(p.resolve, p.reject)
	}()

	return p
}

func (p *Promise[T]) handlePanic() {
	err := recover()
	if err == nil {
		return
	}

	switch v := err.(type) {
	case error:
		p.reject(v)
	default:
		p.reject(fmt.Errorf("%+v", v))
	}
}

// Then registers a callback to be executed when the Promise is fulfilled.
// It returns a new Promise that resolves to the result of the callback.
func (p *Promise[T]) Then(thenFn ThenCallback[T]) *Promise[T] {
	return NewPromise(p.ctx, func(resolveFn ResolveCallback[T], rejectFn RejectCallback) {
		result, err := p.Await() // wait until promise is settled
		if err != nil {
			rejectFn(err)
			return
		}

		nextResult, err := thenFn(result)
		if err != nil {
			rejectFn(err)
			return
		}

		resolveFn(nextResult)
	})
}

// Catch registers a callback to handle errors if the Promise is rejected.
// It returns a new Promise that resolves to the result of the error handler.
func (p *Promise[T]) Catch(errFn CatchCallback[T]) *Promise[T] {
	return NewPromise(p.ctx, func(resolveFn ResolveCallback[T], rejectFn RejectCallback) {
		result, err := p.Await() // wait until promise is settled
		if err != nil {
			nextResult, err := errFn(p.ctx, err)
			if err != nil {
				rejectFn(err)
				return
			}

			resolveFn(nextResult)
			return
		}

		resolveFn(result)
	})
}

// Await blocks until the Promise is settled (fulfilled or rejected) or the context is canceled.
// It returns the resolved value or the error.
func (p *Promise[T]) Await() (T, error) {
	var zero T

	select {
	case <-p.ch:
		if p.err != nil {
			return zero, p.err
		}

		return p.value.Load().(T), nil
	case <-p.ctx.Done(): // Throw If there's any context.Timeout/Cancelled/DeadlineExceeded
		p.reject(p.ctx.Err())

		return zero, p.ctx.Err()
	}
}

// Cancel this promise. Will not do anything if promise is already settled.
func (p *Promise[T]) Cancel() {
	p.cancel(context.Canceled)
}

// IsFulFilled returns true if the Promise has completed successfully.
func (p *Promise[T]) IsFulFilled() bool {
	return p.state.Load() == Fulfilled
}

// IsRejected returns true if the Promise has failed.
func (p *Promise[T]) IsRejected() bool {
	return p.state.Load() == Rejected
}

// IsPending returns true if the Promise is still running.
func (p *Promise[T]) IsPending() bool {
	return p.state.Load() == Pending
}

// IsCancelled returns true if the Promise has cancelled by user.
func (p *Promise[T]) IsCancelled() bool {
	return p.state.Load() == Cancelled
}

func (p *Promise[T]) resolve(value T) {
	if atomic.CompareAndSwapUint32(&p.settled, 0, 1) {
		p.value.Store(value)
		p.state.Store(Fulfilled)
		close(p.ch)
	}
}

func (p *Promise[T]) reject(err error) {
	if atomic.CompareAndSwapUint32(&p.settled, 0, 1) {
		p.err = err
		p.state.Store(Rejected)
		close(p.ch)
	}
}

func (p *Promise[T]) cancel(err error) {
	if atomic.CompareAndSwapUint32(&p.settled, 0, 1) {
		p.err = err
		p.state.Store(Cancelled)
		close(p.ch)
	}
}
