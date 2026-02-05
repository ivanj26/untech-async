package untech_async

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/semaphore"
)

// PromiseMapOpt defines options for the Map function.
type PromiseMapOpt struct {
	Concurrency int64 // Maximum number of concurrent operations
}

// SettledResult represents the result of a promise that has settled.
type SettledResult[T any] struct {
	Value T
	Err   error
}

// ReduceFn defines the function signature for the reduce operation.
type ReduceFn[T, U any] func(accumulator U, currVal T) U

// Race returns a Promise that settles as soon as the first of the input promises settles.
// If the first settling promise fulfills, the returned promise fulfills with that value.
// If the first settling promise rejects, the returned promise rejects with that error.
// If the provided context is canceled before any promise settles, the returned promise rejects with the context error.
// Send panic if the promises input is an empty array
func Race[T any](ctx context.Context, promises ...*Promise[T]) *Promise[T] {
	if len(promises) == 0 {
		panic("promises cannot be empty!")
	}

	return NewPromise(ctx, func(resolveFn ResolveCallback[T], rejectFn RejectCallback) {
		valChan := make(chan T, 1)
		errChan := make(chan error, 1)

		for _, promise := range promises {
			go func(p *Promise[T]) {
				result, err := p.Await()
				if err != nil {
					errChan <- err
					return
				}

				valChan <- result
			}(promise)
		}

		select { // Who is the fastest? timeout/returned value/error?
		case <-ctx.Done():
			rejectFn(ctx.Err())
		case val := <-valChan:
			resolveFn(val)
		case err := <-errChan:
			rejectFn(err)
		}
	})
}

// Map is a utility method when you want to	process multiple inputs as an array, process it using mapping function,
// and use concurrency control to manage the execution.
//
// It waits for all promises to be fulfilled and then applies the provided mapping function to each fulfillment value.
// The promise's fulfillment value is an array of the mapped values at respective positions to the original array.
// If any promise in the array rejects, the returned promise is rejected with the rejection reason.
func Map[T, U any](ctx context.Context, items []T, mapFn func(T) (U, error), opts PromiseMapOpt) *Promise[[]U] {
	if len(items) == 0 {
		return nil
	}

	return NewPromise(ctx, func(resolveFn ResolveCallback[[]U], rejectFn RejectCallback) {
		var (
			wg      sync.WaitGroup
			sem     = semaphore.NewWeighted(opts.Concurrency)
			mtx     sync.Mutex
			values  []U = make([]U, len(items))
			errChan     = make(chan error, 1)
		)

		for i, item := range items {
			wg.Add(1)

			go func(idx int, it T) {
				defer wg.Done()

				sendErr := func(err error) {
					select {
					case errChan <- err:
					default:
					}
				}

				if err := sem.Acquire(ctx, 1); err != nil {
					sendErr(err)
					return
				}
				defer sem.Release(1)

				transformed, err := mapFn(it)
				if err != nil {
					sendErr(err)
					return
				}

				mtx.Lock()
				values[idx] = transformed
				mtx.Unlock()
			}(i, item)
		}

		wg.Wait() // wait for all goroutines to finish

		select {
		case <-ctx.Done():
			rejectFn(ctx.Err())
		case err := <-errChan:
			rejectFn(err)
		default:
			resolveFn(values)
		}
	})
}

// All is a utility method when you want to wait for more than one promise to complete.
//
// The promise's fulfillment value is an array with fulfillment values at respective positions to the original array.
// If any promise in the array rejects, the returned promise is rejected with the rejection reason.
func All[T any](ctx context.Context, promises ...*Promise[T]) *Promise[[]T] {
	if len(promises) == 0 {
		panic("promises cannot be empty!")
	}

	return NewPromise(ctx, func(resolveFn ResolveCallback[[]T], rejectFn RejectCallback) {
		type result struct {
			index int
			val   T
		}

		var (
			wg      sync.WaitGroup
			errChan = make(chan error, 1)

			mtx    sync.Mutex
			values []T = make([]T, len(promises))
		)

		for i, promise := range promises {
			wg.Add(1)
			go func(idx int, p *Promise[T]) {
				defer wg.Done()
				val, err := p.Await()
				if err != nil {
					select {
					case errChan <- err:
					default:
					}
					return
				}

				mtx.Lock()
				defer mtx.Unlock()
				values[idx] = val
			}(i, promise)
		}

		wg.Wait() // wait for all goroutines to finish

		select {
		case <-ctx.Done():
			rejectFn(ctx.Err())
		case err := <-errChan:
			rejectFn(err)
		default:
			resolveFn(values)
		}
	})
}

// AllSettled waits for all promises to settle (either fulfill or reject).
// It panics if the promises slice is empty.
func AllSettled[T any](ctx context.Context, promises []*Promise[T]) *Promise[[]SettledResult[T]] {
	if len(promises) == 0 {
		panic("promises cannot be empty!")
	}

	return NewPromise(ctx, func(resolveFn ResolveCallback[[]SettledResult[T]], rejectFn RejectCallback) {
		type result struct {
			index int
			res   T
			err   error
		}

		valChan := make(chan result, len(promises))

		for i, promise := range promises {
			go func(idx int, p *Promise[T]) {
				val, err := p.Await()
				if err != nil {
					var zero T
					valChan <- result{index: idx, res: zero, err: err}
					return
				} else {
					valChan <- result{index: idx, res: val, err: nil}
				}
			}(i, promise)
		}

		results := make([]SettledResult[T], len(promises))
		resolvedCount := 0

		for {
			select {
			case <-ctx.Done():
				rejectFn(ctx.Err())
				return
			case res := <-valChan:
				results[res.index] = SettledResult[T]{Value: res.res, Err: res.err}
				resolvedCount++

				if resolvedCount == len(promises) {
					resolveFn(results)
					return
				}
			}
		}
	})
}

// Filter waits for all promises to be fulfilled and then filters the results using the provided function.
// It returns a Promise that fulfills with a slice of values for which the filter function returned true.
// If any of the input promises reject, the returned Promise rejects with that error.
// It panics if the promises slice is empty.
func Filter[T any](ctx context.Context, promises []*Promise[T], filterFn func(T) bool) *Promise[[]T] {
	if len(promises) == 0 {
		panic("promises cannot be empty!")
	}

	return NewPromise(ctx, func(resolveFn ResolveCallback[[]T], rejectFn RejectCallback) {
		type result struct {
			index int
			val   T
		}

		valChan := make(chan result, len(promises))
		errChan := make(chan error, 1)

		for i, promise := range promises {
			go func(idx int, p *Promise[T]) {
				val, err := p.Await()
				if err != nil {
					select {
					case errChan <- err:
					default:
					}
					return
				}

				valChan <- result{index: idx, val: val}
			}(i, promise)
		}

		results := make([]T, len(promises))
		resolvedCount := 0

		for {
			select {
			case <-ctx.Done():
				rejectFn(ctx.Err())
				return
			case err := <-errChan:
				rejectFn(err)
				return
			case res := <-valChan:
				results[res.index] = res.val
				resolvedCount++

				if resolvedCount == len(promises) {
					filtered := make([]T, 0, len(promises))
					for _, v := range results {
						if filterFn(v) {
							filtered = append(filtered, v)
						}
					}
					resolveFn(filtered)
					return
				}
			}
		}
	})
}

// Some initiates a race that settles when 'count' of the provided promises fulfill.
// It returns a Promise that fulfills with a slice containing the values of the first 'count' fulfilled promises.
// If fewer than 'count' promises fulfill (i.e., too many reject), the returned Promise rejects with an error.
// It panics if the promises slice is empty.
func Some[T any](ctx context.Context, promises []*Promise[T], count int) *Promise[[]T] {
	if len(promises) == 0 {
		panic("promises cannot be empty!")
	}

	return NewPromise(ctx, func(resolveFn ResolveCallback[[]T], rejectFn RejectCallback) {
		valChan := make(chan T, len(promises))
		errChan := make(chan error, len(promises))

		for _, promise := range promises {
			go func(p *Promise[T]) {
				result, err := p.Await()
				if err != nil {
					errChan <- err
					return
				}

				valChan <- result
			}(promise)
		}

		var (
			result  []T = make([]T, 0, count)
			nbOfErr     = 0
		)

		for {
			select {
			case <-ctx.Done(): // Timeout error? Promise is rejected
				rejectFn(ctx.Err())
				return
			case val := <-valChan: // Collect the value until `count` number
				result = append(result, val)
				if len(result) == count {
					resolveFn(result)
					return
				}
			case _ = <-errChan:
				nbOfErr++

				// If too many rejected promises, it will be immediately
				// and set promise state as rejected
				if nbOfErr > len(promises)-count {
					rejectFn(fmt.Errorf("Too many rejected promises!"))
					return
				}
			}
		}
	})
}

// ReduceParallel waits for all promises to be fulfilled and then reduces the results to a single value using given reduce function.
// It does not guarantee the completion order. If you expect a completion order, you should use Reduce() instead.
//
// If any of the input promises reject, the returned Promise rejects with that error.
// If the promises slice is empty, it resolves with the initialValue.
func ReduceParallel[T, U any](ctx context.Context, promises []*Promise[T], reduceFn ReduceFn[T, U], initialValue U) *Promise[U] {
	return reduce(ctx, promises, reduceFn, initialValue, true)
}

// Reduce waits for all promises to be fulfilled and then reduces the results to a single value using given reduce function.
// Reduce() guarantees the completion order.
//
// If any of the input promises reject, the returned Promise rejects with that error.
// If the promises slice is empty, it resolves with the initialValue.
func Reduce[T, U any](ctx context.Context, promises []*Promise[T], reduceFn ReduceFn[T, U], initialValue U) *Promise[U] {
	return reduce(ctx, promises, reduceFn, initialValue, false)
}

func reduce[T, U any](ctx context.Context, promises []*Promise[T], reduceFn ReduceFn[T, U], initialValue U, isParallel bool) *Promise[U] {
	if len(promises) == 0 {
		return NewPromise(ctx, func(resolveFn ResolveCallback[U], rejectFn RejectCallback) {
			resolveFn(initialValue)
		})
	}

	return NewPromise(ctx, func(resolveFn ResolveCallback[U], rejectFn RejectCallback) {
		type result struct {
			idx int
			val T
		}

		valChan := make(chan result, len(promises))
		errChan := make(chan error, 1)

		for i, promise := range promises {
			go func(idx int, p *Promise[T]) {
				val, err := p.Await()
				if err != nil {
					errChan <- err
					return
				}

				valChan <- result{idx: idx, val: val}
			}(i, promise)
		}

		resolvedCount := 0
		results := make([]T, len(promises))
		acc := initialValue

		for {
			select {
			case <-ctx.Done():
				rejectFn(ctx.Err())
				return
			case err := <-errChan:
				rejectFn(err)
				return
			case res := <-valChan:
				resolvedCount++

				if isParallel {
					acc = reduceFn(acc, res.val)

					if resolvedCount == len(promises) {
						resolveFn(acc)
						return
					}
				} else {
					results[res.idx] = res.val

					if resolvedCount == len(promises) {
						for _, v := range results {
							acc = reduceFn(acc, v)
						}
						resolveFn(acc)
						return
					}
				}
			}
		}
	})
}
