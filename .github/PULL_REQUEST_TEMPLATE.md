## 🧾 Summary
<!-- What does this PR change? Keep it concise and concrete. -->
Provide a short and concise summary when you want to raise a PR.

## 🏷️ Type of Change
- [ ] Bug fix
- [ ] New core/util function
- [ ] Breaking API change
- [ ] Performance improvement
- [ ] Refactor / internal cleanup
- [ ] Documentation / comments

## 🎯 Motivation
<!-- Why is this change needed? What problem does it solve? -->
Set a motivation on the change that you want to bring.

## 🔗 Related Issue
<!-- Link to related issue(s) -->
What is the issue that you want to solve?
If it is intended for improvement idea on a new feature, put `-` mark.
Otherwise, please attach the related issue link.

## 🧩 Examples
<!-- Provide minimal, focused examples if applicable -->
```go
p := untech_async.NewPromise(context.Background(), func(resolve untech_async.ResolveCallback[string], reject untech_async.RejectCallback) {
		resolve("hello world")
	})

result, err := p.Await()
if err != nil {
    fmt.Printf("Promise rejected: %v\n", err)
    return
}

fmt.Printf("Promise fulfilled with result: %s\n", result)
```