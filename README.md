<p align="center">
  <a href="https://github.com/ivanj26/untech-async/actions"><img src="https://github.com/ivanj26/untech-async/workflows/Unit%20Test/badge.svg" alt="Tests"></a>
  <a href="https://github.com/ivanj26/untech-async/actions"><img src="https://github.com/ivanj26/untech-async/workflows/Code%20Style/badge.svg" alt="Code Style"></a>
  <a href="https://github.com/ivanj26/untech-async/releases"><img src="https://img.shields.io/github/v/release/ivanj26/untech-async" alt="Latest Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/ivanj26/untech-async" alt="License"></a>
</p>

# untech<sup>2</sup>-async
untech<sup>2</sup>-async is a promise-like library built for Go, technically inspired by the fully-featured JavaScript promise library, `bluebird.js`.

It is designed for JavaScript/TypeScript developers who are new to Go. Philosophically, the name is taken from the President of Indonesia's jargon, ```"Hey, Antek-antek asing!"```, which sounds similar to ```"Hey, untech-untech async!"``` to non-native Indonesian speakers.

## Install

```sh
go get github.com/ivanj26/untech-async
```

## Quickstart

### Basic Usage: Creating Promise

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ivanj26/untech-async"
)

func main() {
	p := untech_async.NewPromise(context.Background(), func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
		fmt.Println("Doing some async work...")
		time.Sleep(1 * time.Second)
		resolve("Hello from Promise!")
	})

	fmt.Println("Waiting for promise to complete...")

	// Await blocks until the promise is fulfilled or rejected.
	result, err := p.Await()
	if err != nil {
		fmt.Printf("Promise rejected with error: %v\n", err)
		return
	}

	fmt.Printf("Promise fulfilled with result: %s\n", result)
}
```

### Chaining with `Then`

The `.Then()` method allows you to execute asynchronous operations in sequence. It takes the result of the previous promise, processes it, and returns a new promise with the new result.

```go
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ivanj26/untech-async"
)

func main() {
	p := untech_async.NewPromise(context.Background(), func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
		time.Sleep(500 * time.Millisecond)
		resolve("hello world")
	})

	// Chain operations using .Then()
	p2 := p.
                Then(func(value string) (string, error) {
                        fmt.Printf("First 'Then' received: %s\n", value)
                        return strings.ToUpper(value), nil
                }).
                Then(func(value string) (string, error) {
                        fmt.Printf("Second 'Then' received: %s\n", value)
                        return "Processed: " + value, nil
	        })

	result, err := p2.Await()
	if err != nil {
		fmt.Printf("Chained promise rejected: %v\n", err)
		return
	}

	fmt.Printf("Final result: %s\n", result)
}
```

### Error Handling with `Catch`

If any promise in a chain is rejected, it will jump to the next `.Catch()` block. You can use `Catch` to handle errors, recover from them, and continue the chain with a new value.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivanj26/untech-async"
)

func main() {
	p := untech_async.NewPromise(context.Background(), func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
		time.Sleep(500 * time.Millisecond)
		reject(errors.New("something went wrong"))
	})

	// Use .Catch() to recover from the error and provide a default value.
	pRecover := p.Catch(func(ctx context.Context, err error) (string, error) {
		fmt.Printf("Caught error: %v\n", err)
		return "recovered value", nil
	})

	result, err := pRecover.Await()
	if err != nil {
		fmt.Printf("This should not happen, but got error: %v\n", err)
	} else {
		fmt.Printf("Final result after recovery: %s\n", result)
	}
}
```

### Context and Cancellation

Promises are integrated with Go's `context` package. You can cancel promises through context timeouts, deadlines, or manual cancellation.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ivanj26/untech-async"
)

func main() {
	// To set a timeout for your Promise object, call context.Timeout(), and pass it to Promise constructor.
        // Once timeout exceeded, it will response an error.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	p := untech_async.NewPromise(ctx, func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
		fmt.Println("Starting a long-running task (1 second)...")
		time.Sleep(1 * time.Second)
		resolve("done")
	})

	_, err := p.Await()
	if err != nil {
		fmt.Printf("Promise failed: %v\n", err) // Promise failed: context deadline exceeded
	}

	// Manual Cancellation Example
	p2 := untech_async.NewPromise(context.Background(), func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
		time.Sleep(1 * time.Second)
		resolve("done")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		p2.Cancel()
	}()

	_, err2 := p2.Await()
	if err2 != nil {
		fmt.Printf("Promise 2 failed: %v\n", err2) // Promise 2 failed: context canceled
	}
}
```

## Utility

`untech-async` provides utility functions to work with multiple promises at once.

### `Promise.All`

`All` returns a promise that fulfills when **all** input promises have fulfilled. If any promise rejects, `All` immediately rejects.

```go
p1 := untech_async.NewPromise(ctx, func(r untech_async.ResolveCallback[string], _ untech_async.RejectCallback) { r("one") })
p2 := untech_async.NewPromise(ctx, func(r untech_async.ResolveCallback[string], _ untech_async.RejectCallback) { r("two") })

all := untech_async.All(ctx, p1, p2)
results, _ := all.Await() // results is []string{"one", "two"}
```

### `Promise.Race`

`Race` returns a promise that settles as soon as the **first** input promise settles.

```go
pSlow := untech_async.NewPromise(ctx, func(r untech_async.ResolveCallback[string], _ untech_async.RejectCallback) { time.Sleep(100*time.Millisecond); r("slow") })
pFast := untech_async.NewPromise(ctx, func(r untech_async.ResolveCallback[string], _ untech_async.RejectCallback) { time.Sleep(10*time.Millisecond); r("fast") })

race := untech_async.Race(ctx, pSlow, pFast)
result, _ := race.Await() // result is "fast"
```

### `Promise.AllSettled`

`AllSettled` returns a promise that fulfills after all input promises have settled. It provides a slice of `SettledResult` structs describing the outcome of each promise.

```go
p1 := untech_async.NewPromise(ctx, func(r untech_async.ResolveCallback[int], _ untech_async.RejectCallback) { r(1) })
p2 := untech_async.NewPromise(ctx, func(_ untech_async.ResolveCallback[int], r untech_async.RejectCallback) { r(errors.New("failure")) })

allSettled := untech_async.AllSettled(ctx, []*untech_async.Promise[int]{p1, p2})
results, _ := allSettled.Await()
// results[0] = {Value: 1, Err: nil}
// results[1] = {Value: 0, Err: "failure"}
```

## Utility Functions

The library also includes other powerful utilities for processing collections:

-   **`Map(items, mapFn, opts)`**: Processes a slice of items concurrently with a configurable concurrency limit.
-   **`Some(promises, count)`**: Fulfills when `count` promises have fulfilled.
-   **`Filter(promises, filterFn)`**: Filters promise results based on a predicate.
-   **`Reduce(promises, reduceFn, initial)`**: Reduces promise results in order.
-   **`ReduceParallel(promises, reduceFn, initial)`**: Reduces promise results as they complete.

Check the `util_test.go` file for detailed examples of their usage.

