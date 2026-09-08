## Exercise
### Q1: What is atomic operation?

不可分割的操作行為，有些指令行為背後可能由多行組合語言指令組成，為了避免發生指令交錯執行造成結果不如預期，所以需要用 atomic. 來確保該指令會完整執行才輪到其他的

### Q2: Does the example below have concurrency issues? Why? 
Yes. There are two concurrency issues:

1. Data race / race condition
    - `a = a + 1` is not an atomic operation. It reads `a`, adds 1, then writes the result back to `a`.
    - If many goroutines execute this statement at the same time, they may read the same old value and overwrite each other's updates.
2. Goroutine lifecycle / synchronization issue
    - The `WaitGroup` in `main` only waits for the outer goroutine that calls `increment()`.
    - It does not wait for the goroutines created inside `increment()`, so `main` may print the final value and exit before those inner goroutines finish.

```go
var a int64

func main() {
    var wg sync.WaitGroup
    wg.Add(1) // wait for only one outer goroutine
    
    go func() {
        increment()
        wg.Done() // mark the outer goroutine as done
    }()

    wg.Wait()

    fmt.Println("Final value of a:", a)
}

func increment() {
    // This loop starts many goroutines.
    // These goroutines run concurrently and may access "a" at the same time.
    // "a = a + 1" is not atomic, so multiple goroutines may overwrite
    // each other's updates.
    // Also, the WaitGroup in main does not wait for these inner goroutines.
    for i := 0; i < 100000000; i++ {
        go func() {
             a = a + 1    
        }()
    }
}
```

### Q3: What do the **`wg.Add(1)`** and **`wg.Done()`** do in the above statement? And what does the **`1`** repersent?

`wg.Add(1)` increases the WaitGroup counter by 1. It tells the WaitGroup that there is one goroutine or task that needs to be waited for.

`wg.Done()` decreases the WaitGroup counter by 1. It means that one goroutine or task has finished. It is the same as calling:

```go
wg.Add(-1)
```

The `1` in `wg.Add(1)` represents the number of tasks added to the WaitGroup counter. In the Q2 code, it only represents the outer goroutine that calls `increment()`. It does not represent all goroutines created inside `increment()`.

When `wg.Wait()` is called, the main goroutine blocks until the WaitGroup counter becomes 0.



### Q4: Please define a Counter struct with an integer field and a sync.Mutex, then implement a function to increment the counter safely.

Since value increasement is not atomic, so we need to use mutex to protect counter increasement safety

```go
package main

import "sync"

type Counter struct {
    value int
    mu    sync.Mutex // lock
}

func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock() // do after this function end

    c.value++
}

func (c *Counter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()

    return c.value
}
```

The `value` field is the shared resource. Since `c.value++` is not atomic, we use `mu.Lock()` before modifying it and `mu.Unlock()` after modifying it. This makes sure only one goroutine can increment the counter at a time.

The methods use a pointer receiver, `*Counter`, because we want to modify the original `Counter` value instead of modifying a copy. The `Value()` method also uses the mutex, because reading shared data should be protected when other goroutines may write to it at the same time.



### Q5: Why is there a fetal error in following code?
Because `intChan` is only declared but not initialized with `make`, its zero value is `nil`. `fmt.Println(intChan)` only prints the current value of the channel, so it prints `<nil>`.

The real problem is this line:

```go
intChan <- 10
```

Sending data to a nil channel does not succeed and does not return an error directly. Instead, it blocks forever. Since the `main` goroutine is the only goroutine in this program, the Go runtime detects that all goroutines are asleep and reports a deadlock:

```text
fatal error: all goroutines are asleep - deadlock!
```

To fix it, initialize the channel with `make` and make sure there is a receiver:

```go
intChan := make(chan int)

go func() {
    fmt.Println(<-intChan)
}()

intChan <- 10
```

```go=
func main() {
    var intChan chan int
    fmt.Println(intChan) // format output
    // send 10 to intChan channel
    intChan <- 10
}
```

the result output is

```
<nil>
fatal error: all goroutines are asleep - deadlock!
```


### Q6: In Topic: **Select**-**Setup Timeout mechanism**, We offered a sample code that showed how to setup **`timeout`** with **`select`**. Actually, there is a potentail error (Hint: memory leak) because of **`time.After`** usage. Please describe the reason for the error and how to fix it.

original code

```go
func main() {
    timeout := make(chan bool, 1)
    go func() {
        time.Sleep(2 * time.Second)
        timeout <- true
    }()
    ch := make(chan int)
    select {
    case <-ch: // it wont be ready forever since no goroutine send data to it
    case <-timeout:
        fmt.Println("Open5GS")
    case <-time.After(time.Second * 1): //Create a Read-Only Channel which will receive time.Time value
        fmt.Println("free5GC")
    }
}
```

The problem is caused by this line:

```go
case <-time.After(time.Second * 1):
```

When the program enters the `select`, Go evaluates every case expression first. At that time, `time.After(time.Second * 1)` is called immediately. It creates a timer and returns a receive-only channel. The channel is created immediately, but the `time.Time` value is sent to that channel after 1 second.

According to the official Go `time.After` documentation, `time.After(d)` waits for the duration to pass and then sends the current time on the returned channel. It is equivalent to `time.NewTimer(d).C`. Reference: https://pkg.go.dev/time#After

If this case is selected:

```go
case <-time.After(time.Second * 1):
    fmt.Println("free5GC")
```

then the timer has a receiver. `case <-time.After(...)` means the `select` receives a value from the timer channel, but discards that value. It is similar to:

```go
case _ = <-time.After(time.Second * 1):
```

If we want to keep the received value, we can write:

```go
case t := <-time.After(time.Second * 1):
    fmt.Println(t)
```

However, if another case is ready first, for example `case <-timeout:`, then the `select` exits without receiving from the `time.After` channel. The timer created by `time.After` is still active until it fires.

Example:

```text
t = 0s
main enters select
time.After(1s) creates a timer and returns a channel

t = 0.5s
timeout becomes ready
select chooses case <-timeout
select exits

t = 1s
the time.After timer fires
but the select has already ended, so no code receives from that timer channel
```

This does not mean the timer survives after the whole `main` function exits. If `main` returns, the whole process exits. The real issue is when this pattern is used in a long-running program or inside a loop: many unused timers may accumulate before they fire.

For Go versions before 1.23, the official documentation warned that the underlying timer would not be recovered by the garbage collector until it fired. The Go 1.23 timer wiki also says that before Go 1.23, unstopped timers could not be garbage collected until the timer went off. References:

- https://pkg.go.dev/time#After
- https://go.dev/wiki/Go123Timer

To fix this in older Go versions or when we want explicit timer control, use `time.NewTimer` and stop the timer when another case is selected:

```go
timer := time.NewTimer(time.Second * 1)

select {
case <-ch:
    if !timer.Stop() {
        select {
        case <-timer.C:
        default:
        }
    }
case <-timeout:
    if !timer.Stop() {
        select {
        case <-timer.C:
        default:
        }
    }
    fmt.Println("Open5GS")
case <-timer.C:
    fmt.Println("free5GC")
}
```

In Go 1.23 or later, unreferenced unstopped timers can be garbage collected, so this memory leak issue is improved. But for this lab question, the expected answer is still: `time.After` creates a timer that cannot be stopped directly; if another `select` case wins first, the timer may stay alive until it fires, so use `time.NewTimer` plus `Stop()` when explicit cleanup is needed.

---

### You can reference the answer of this exercise at [ans/Answer.md](https://github.com/KunLee76/free5GCLab/blob/feat/lab1/lab1/ans/Answer.md)

