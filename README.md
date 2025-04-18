# Periodic

Periodic is a lightweight library for managing periodic tasks in your Go applications.

## Features

- Simple and intuitive interface for scheduling periodic tasks:
  - `Start()` for (re-)starting the periodic execution;
  - `Stop()` to gracefully interrupt the execution by cancelling the context;
  - `Wait()` to wait for the running tasks to terminate;
  - `Error()` to consult the termination reason.
- A `Ticker` interface and an implementation that ticks on start and closes the channel on destruction.
- A `TestTicker` implementation of the `Ticker` interface that allows for sending a code controlled ticks.
- A list of task wrappers, such as `NoOverlap`, `WithRetry` and others.

## Example

The example shows a periodic task, that prints "tick" 3 times once a second.

```go
package main

import (
  "fmt"
  "time"

  "github.com/parametalol/periodic"
)

func tick() { fmt.Println("tick") }

func main() {
  task := periodic.NewTask("tick task", time.Second, tick)
  task.Start()
  time.Sleep(2500 * time.Millisecond)
  task.Stop()
  task.Wait()
}
```
