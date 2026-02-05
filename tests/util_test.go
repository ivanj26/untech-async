package tests

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	untech_async "github.com/ivanj26/untech-async"
	"github.com/stretchr/testify/assert"
)

func TestRaceSuccess(t *testing.T) {
	ctx := context.Background()

	p1 := untech_async.NewPromise(ctx, func(resolveFn untech_async.ResolveCallback[string], rejectFn untech_async.RejectCallback) {
		time.Sleep(time.Millisecond * 500)
		resolveFn("slower")
	})
	p2 := untech_async.NewPromise(ctx, func(resolveFn untech_async.ResolveCallback[string], rejectFn untech_async.RejectCallback) {
		time.Sleep(time.Millisecond * 100)
		resolveFn("faster")
	})

	p := untech_async.Race(ctx, p1, p2)

	val, err := p.Await()
	assert.NoError(t, err)
	assert.NotNil(t, val)
	assert.Equal(t, "faster", val)
}

func TestRaceReject(t *testing.T) {
	expectedErr := errors.New("this is error!")

	p1 := untech_async.NewPromise(context.Background(), func(resolveFn untech_async.ResolveCallback[any], rejectFn untech_async.RejectCallback) {
		time.Sleep(time.Millisecond * 100)
		resolveFn(nil)
	})
	p2 := untech_async.NewPromise(context.Background(), func(resolveFn untech_async.ResolveCallback[any], rejectFn untech_async.RejectCallback) {
		rejectFn(expectedErr)
	})

	p := untech_async.Race(context.Background(), p1, p2)

	val, err := p.Await()
	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, val)
}

func TestAll(t *testing.T) {
	ctx := context.Background()

	t.Run("Should wait for all promises to resolve", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			time.Sleep(50 * time.Millisecond)
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			time.Sleep(10 * time.Millisecond)
			resolve(2)
		})
		p3 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			time.Sleep(30 * time.Millisecond)
			resolve(3)
		})

		p := untech_async.All(ctx, p1, p2, p3)
		results, err := p.Await()

		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("Should reject if any promise rejects", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(errors.New("failed"))
		})

		p := untech_async.All(ctx, p1, p2)
		_, err := p.Await()

		assert.Error(t, err)
		assert.EqualError(t, err, "failed")
	})

	t.Run("Should reject if context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		p1 := untech_async.NewPromise(context.Background(), func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			time.Sleep(100 * time.Millisecond)
			resolve(1)
		})

		p := untech_async.All(ctx, p1)
		cancel()

		_, err := p.Await()

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Should reject if context is timeout", func(t *testing.T) {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(10)*time.Millisecond)
		p1 := untech_async.NewPromise(context.Background(), func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			time.Sleep(100 * time.Millisecond)
			resolve(1)
		})

		p := untech_async.All(ctx, p1)

		_, err := p.Await()

		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
	})

	t.Run("Should panic if promises slice is empty", func(t *testing.T) {
		assert.PanicsWithValue(t, "promises cannot be empty!", func() {
			untech_async.All[int](ctx)
		})
	})
}

func TestAllSettled(t *testing.T) {
	ctx := context.Background()

	t.Run("Should fulfill when all promises fulfill", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(2)
		})

		promises := []*untech_async.Promise[int]{p1, p2}
		p := untech_async.AllSettled(ctx, promises)
		results, err := p.Await()

		assert.NoError(t, err)
		assert.Equal(t, []untech_async.SettledResult[int]{
			{Value: 1, Err: nil},
			{Value: 2, Err: nil},
		}, results)
	})

	t.Run("Should fulfill even if some promises reject", func(t *testing.T) {
		expectedErr := errors.New("rejection")
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(expectedErr)
		})

		promises := []*untech_async.Promise[int]{p1, p2}
		p := untech_async.AllSettled(ctx, promises)
		results, err := p.Await()

		assert.NoError(t, err, "AllSettled promise should fulfill even with rejected promises")
		assert.Len(t, results, 2)
		assert.Equal(t, 1, results[0].Value)
		assert.NoError(t, results[0].Err)
		assert.Equal(t, 0, results[1].Value) // zero value for int
		assert.Error(t, results[1].Err)
		assert.Equal(t, expectedErr, results[1].Err)
	})

	t.Run("Should fulfill when all promises reject", func(t *testing.T) {
		err1 := errors.New("rejection 1")
		err2 := errors.New("rejection 2")
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(err1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(err2)
		})

		promises := []*untech_async.Promise[int]{p1, p2}
		p := untech_async.AllSettled(ctx, promises)
		results, err := p.Await()

		assert.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, 0, results[0].Value)
		assert.Equal(t, err1, results[0].Err)
		assert.Equal(t, 0, results[1].Value)
		assert.Equal(t, err2, results[1].Err)
	})

	t.Run("Should panic if promises slice is empty", func(t *testing.T) {
		assert.PanicsWithValue(t, "promises cannot be empty!", func() {
			untech_async.AllSettled(ctx, []*untech_async.Promise[int]{})
		})
	})

	t.Run("Should reject if context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		p1 := untech_async.NewPromise(context.Background(), func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			time.Sleep(100 * time.Millisecond)
			resolve(1)
		})

		p := untech_async.AllSettled(ctx, []*untech_async.Promise[int]{p1})
		cancel() // cancel immediately

		_, err := p.Await()

		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestSome(t *testing.T) {
	ctx := context.Background()

	t.Run("Should resolve with first N fulfilled promises", func(t *testing.T) {
		promises := make([]*untech_async.Promise[int], 3)
		for i := range 3 {
			idx := i
			promises[i] = untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
				time.Sleep(time.Duration(idx+1) * 10 * time.Millisecond)
				resolve(idx + 1)
			})
		}

		p := untech_async.Some(ctx, promises, 2)
		results, err := p.Await()

		assert.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Contains(t, results, 1)
		assert.Contains(t, results, 2)
	})

	t.Run("Should reject if too many promises reject", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(errors.New("failed 1"))
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(errors.New("failed 2"))
		})
		p3 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(3)
		})

		promises := []*untech_async.Promise[int]{p1, p2, p3}

		p := untech_async.Some(ctx, promises, 2)
		_, err := p.Await()

		assert.Error(t, err)
		assert.EqualError(t, err, "Too many rejected promises!")
	})

	t.Run("Should resolve even if there is a rejected promise, as long as number of rejected promises is less than provided count", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(errors.New("failed 1"))
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(errors.New("failed 2"))
		})
		p3 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(3)
		})
		p4 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(4)
		})

		promises := []*untech_async.Promise[int]{p1, p2, p3, p4}

		p := untech_async.Some(ctx, promises, 2)
		results, err := p.Await()

		assert.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Contains(t, results, 3)
		assert.Contains(t, results, 4)
	})
}

func TestFilter(t *testing.T) {
	ctx := context.Background()

	t.Run("Should filter integers correctly", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(2)
		})
		p3 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(3)
		})
		p4 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(4)
		})

		promises := []*untech_async.Promise[int]{p1, p2, p3, p4}
		filterFn := func(val int) bool {
			return val%2 == 0 // Keep even numbers
		}

		resultPromise := untech_async.Filter(ctx, promises, filterFn)
		result, err := resultPromise.Await()

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, []int{2, 4}, result)
	})

	t.Run("Should reject if one promise rejects", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(errors.New("failed"))
		})

		promises := []*untech_async.Promise[int]{p1, p2}
		filterFn := func(val int) bool { return true }

		resultPromise := untech_async.Filter(ctx, promises, filterFn)
		_, err := resultPromise.Await()

		assert.Error(t, err)
		assert.EqualError(t, err, "failed")
	})

	t.Run("Should panic if promises slice is empty", func(t *testing.T) {
		promises := []*untech_async.Promise[int]{}
		filterFn := func(val int) bool { return true }

		assert.PanicsWithValue(t, "promises cannot be empty!", func() {
			untech_async.Filter(ctx, promises, filterFn)
		})
	})
}

func TestReduce_Parallel(t *testing.T) {
	ctx := context.Background()

	t.Run("Should reduce integers correctly (sum)", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(2)
		})
		p3 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(3)
		})

		promises := []*untech_async.Promise[int]{p1, p2, p3}
		reduceFn := func(acc int, curr int) int {
			return acc + curr
		}

		resultPromise := untech_async.ReduceParallel(ctx, promises, reduceFn, 0)
		result, err := resultPromise.Await()

		assert.NoError(t, err)
		assert.Equal(t, 6, result)
	})

	t.Run("Should reduce strings based on completion order", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
			time.Sleep(50 * time.Millisecond) // 50ms
			resolve("A")
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
			time.Sleep(10 * time.Millisecond) // 10ms
			resolve("B")
		})
		p3 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
			time.Sleep(30 * time.Millisecond) // 30ms
			resolve("C")
		})

		promises := []*untech_async.Promise[string]{p1, p2, p3}
		reduceFn := func(acc string, curr string) string {
			return acc + curr
		}

		// Initial value is empty string
		resultPromise := untech_async.ReduceParallel(ctx, promises, reduceFn, "")
		result, err := resultPromise.Await()

		assert.NoError(t, err)

		// Expect "BCA" because B finishes first (10ms), then C (30ms), then A (50ms)
		assert.Equal(t, "BCA", result)
	})

	t.Run("Should return initial value for empty promises", func(t *testing.T) {
		promises := []*untech_async.Promise[int]{}
		reduceFn := func(acc int, curr int) int { return acc + curr }

		resultPromise := untech_async.ReduceParallel(ctx, promises, reduceFn, 10)
		result, err := resultPromise.Await()

		assert.NoError(t, err)
		assert.Equal(t, 10, result)
	})

	t.Run("Should reject if one promise rejects", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(errors.New("failed"))
		})

		promises := []*untech_async.Promise[int]{p1, p2}
		reduceFn := func(acc int, curr int) int { return acc + curr }

		resultPromise := untech_async.ReduceParallel(ctx, promises, reduceFn, 0)
		_, err := resultPromise.Await()

		assert.Error(t, err)
		assert.EqualError(t, err, "failed")
	})
}

func TestMap(t *testing.T) {
	ctx := context.Background()

	t.Run("Should map all items successfully", func(t *testing.T) {
		items := []int{1, 2, 3}
		mapFn := func(item int) (string, error) {
			return fmt.Sprintf("item-%d", item), nil
		}
		opts := untech_async.PromiseMapOpt{Concurrency: 2}

		p := untech_async.Map(ctx, items, mapFn, opts)
		result, err := p.Await()

		assert.NoError(t, err)
		assert.Equal(t, []string{"item-1", "item-2", "item-3"}, result)
	})

	t.Run("Should reject if mapping function returns an error", func(t *testing.T) {
		items := []int{1, 2, 3}
		expectedErr := errors.New("map error")
		mapFn := func(item int) (string, error) {
			if item == 2 {
				return "", expectedErr
			}
			return fmt.Sprintf("item-%d", item), nil
		}
		opts := untech_async.PromiseMapOpt{Concurrency: 2}

		p := untech_async.Map(ctx, items, mapFn, opts)
		_, err := p.Await()

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("Should handle multiple errors without deadlocking", func(t *testing.T) {
		items := []int{1, 2, 3, 4}
		err1 := errors.New("map error 1")
		err2 := errors.New("map error 2")

		mapFn := func(item int) (string, error) {
			if item == 2 {
				return "", err1
			}
			if item == 3 {
				return "", err2
			}
			return fmt.Sprintf("item-%d", item), nil
		}
		opts := untech_async.PromiseMapOpt{Concurrency: 4}

		p := untech_async.Map(ctx, items, mapFn, opts)
		_, err := p.Await()

		assert.Error(t, err)
		// The first error to be sent wins, which is racy.
		// So we check if it's one of the expected errors.
		assert.True(t, errors.Is(err, err1) || errors.Is(err, err2))
	})

	t.Run("Should return nil for empty input slice", func(t *testing.T) {
		items := []int{}
		mapFn := func(item int) (string, error) {
			return "", nil
		}
		opts := untech_async.PromiseMapOpt{Concurrency: 2}

		p := untech_async.Map(ctx, items, mapFn, opts)
		assert.Nil(t, p)
	})

	t.Run("Should reject if context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		items := []int{1, 2, 3}
		mapFn := func(item int) (string, error) {
			time.Sleep(50 * time.Millisecond)
			return fmt.Sprintf("item-%d", item), nil
		}
		opts := untech_async.PromiseMapOpt{Concurrency: 1}

		p := untech_async.Map(ctx, items, mapFn, opts)

		// Cancel after a short time, before the first item is processed
		time.Sleep(10 * time.Millisecond)
		cancel()

		_, err := p.Await()

		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("Should reject if context times out", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		items := []int{1, 2, 3, 4}
		mapFn := func(item int) (string, error) {
			time.Sleep(50 * time.Millisecond)
			return fmt.Sprintf("item-%d", item), nil
		}
		opts := untech_async.PromiseMapOpt{Concurrency: 2}

		p := untech_async.Map(ctx, items, mapFn, opts)
		_, err := p.Await()

		assert.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("Should respect concurrency option", func(t *testing.T) {
		items := []int{1, 2, 3, 4, 5}
		concurrency := int64(2)
		var active int64
		var maxActive int64
		var mtx sync.Mutex

		mapFn := func(item int) (string, error) {
			mtx.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mtx.Unlock()

			time.Sleep(50 * time.Millisecond)

			mtx.Lock()
			active--
			mtx.Unlock()
			return fmt.Sprintf("item-%d", item), nil
		}
		opts := untech_async.PromiseMapOpt{Concurrency: concurrency}

		p := untech_async.Map(ctx, items, mapFn, opts)
		_, err := p.Await()

		assert.NoError(t, err)
		assert.Equal(t, concurrency, maxActive, "Expected maximum concurrency to be respected")
	})
}

func TestReduce(t *testing.T) {
	ctx := context.Background()

	t.Run("Should reduce integers correctly (sum)", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(2)
		})
		p3 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(3)
		})

		promises := []*untech_async.Promise[int]{p1, p2, p3}
		reduceFn := func(acc int, curr int) int {
			return acc + curr
		}

		resultPromise := untech_async.Reduce(ctx, promises, reduceFn, 0)
		result, err := resultPromise.Await()

		assert.NoError(t, err)
		assert.Equal(t, 6, result)
	})

	t.Run("Should reduce strings based on input order", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
			time.Sleep(50 * time.Millisecond) // 50ms
			resolve("A")
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
			time.Sleep(10 * time.Millisecond) // 10ms
			resolve("B")
		})
		p3 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
			time.Sleep(30 * time.Millisecond) // 30ms
			resolve("C")
		})

		promises := []*untech_async.Promise[string]{p1, p2, p3}
		reduceFn := func(acc string, curr string) string {
			return acc + curr
		}

		// Initial value is empty string
		resultPromise := untech_async.Reduce(ctx, promises, reduceFn, "")
		result, err := resultPromise.Await()

		assert.NoError(t, err)

		// Expect "ABC" because Reduce preserves input order: p1("A"), p2("B"), p3("C")
		// Even though B finishes first, then C, then A.
		assert.Equal(t, "ABC", result)
	})

	t.Run("Should return initial value for empty promises", func(t *testing.T) {
		promises := []*untech_async.Promise[int]{}
		reduceFn := func(acc int, curr int) int { return acc + curr }

		resultPromise := untech_async.Reduce(ctx, promises, reduceFn, 10)
		result, err := resultPromise.Await()

		assert.NoError(t, err)
		assert.Equal(t, 10, result)
	})

	t.Run("Should reject if one promise rejects", func(t *testing.T) {
		p1 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(1)
		})
		p2 := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			reject(errors.New("failed"))
		})

		promises := []*untech_async.Promise[int]{p1, p2}
		reduceFn := func(acc int, curr int) int { return acc + curr }

		resultPromise := untech_async.Reduce(ctx, promises, reduceFn, 0)
		_, err := resultPromise.Await()

		assert.Error(t, err)
		assert.EqualError(t, err, "failed")
	})
}
