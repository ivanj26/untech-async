package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	untech_async "github.com/ivanj26/untech-async"
	"github.com/stretchr/testify/assert"
)

func TestPromise_Then(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("this is error!")

	p1 := untech_async.NewPromise(ctx, func(resolveFn untech_async.ResolveCallback[string], rejectFn untech_async.RejectCallback) {
		resolveFn("Hello, ")
	})
	p2 := p1.Then(func(value string) (string, error) {
		var sb strings.Builder
		sb.WriteString(value)
		sb.WriteString("world!")

		return sb.String(), nil
	})
	p3 := p2.Then(func(value string) (string, error) {
		return "", expectedErr
	})

	val, err := p1.Await()
	assert.NoError(t, err)
	assert.NotNil(t, val)
	assert.Equal(t, "Hello, ", val)

	val, err = p2.Await()
	assert.NoError(t, err)
	assert.NotNil(t, val)
	assert.Equal(t, "Hello, world!", val)

	_, err = p3.Await()
	assert.EqualError(t, err, expectedErr.Error())
}

func TestPromise_Catch(t *testing.T) {
	expectedErr := errors.New("this is error!")

	p1 := untech_async.NewPromise(context.Background(), func(resolveFn untech_async.ResolveCallback[any], rejectFn untech_async.RejectCallback) {
		rejectFn(expectedErr)
	})

	val, err := p1.Await()

	assert.True(t, p1.IsRejected())
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, val)
}

func TestChaining(t *testing.T) {
	ctx := context.Background()

	var data int = 0
	var w sync.WaitGroup
	w.Add(3)

	untech_async.
		NewPromise(ctx, func(resolve untech_async.ResolveCallback[int], reject untech_async.RejectCallback) {
			resolve(2)
		}).
		Then(func(value int) (int, error) {
			w.Done()
			return value + 1, nil
		}).
		Then(func(value int) (int, error) {
			w.Done()

			if value == 3 {
				return 0, fmt.Errorf("it worked")
			}
			return 0, nil
		}).
		Catch(func(c context.Context, err error) (int, error) {
			if err.Error() == "it worked" {
				data = 3
			}
			w.Done()
			return 0, nil
		})

	w.Wait()
	assert.Equal(t, 3, data)
}

func TestIsPending(t *testing.T) {
	p := untech_async.NewPromise(context.Background(), func(resolveFn untech_async.ResolveCallback[any], rejectFn untech_async.RejectCallback) {
		time.Sleep(time.Duration(1) * time.Second)
		resolveFn(nil)
	})

	assert.True(t, p.IsPending())
}

func TestIsFulfilled(t *testing.T) {
	p := untech_async.NewPromise(context.Background(), func(resolveFn untech_async.ResolveCallback[any], rejectFn untech_async.RejectCallback) {
		time.Sleep(time.Duration(1) * time.Second)
		resolveFn(true)
	})
	p.Await()

	assert.True(t, p.IsFulFilled())
}

func TestIsCancelled(t *testing.T) {
	p := untech_async.NewPromise(context.Background(), func(resolveFn untech_async.ResolveCallback[any], rejectFn untech_async.RejectCallback) {
		time.Sleep(time.Duration(1) * time.Second)
		resolveFn(true)
	})
	p.Cancel()
	p.Await() // wait until ch receive cancel signal

	assert.True(t, p.IsCancelled())
}

func TestIsRejected(t *testing.T) {
	p := untech_async.NewPromise(context.Background(), func(resolveFn untech_async.ResolveCallback[any], rejectFn untech_async.RejectCallback) {
		rejectFn(errors.New("error"))
	})
	p.Await()

	assert.True(t, p.IsRejected())
}
