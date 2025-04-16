# Periodic

Periodic is a lightweight library for managing periodic tasks in your Go applications.

## Features

- Simple and intuitive interface for scheduling periodic tasks:
  - `Start()` for (re-)starting the periodic execution;
  - `Stop()` to gracefully interrupt the execution by cancelling the context;
  - `Wait()` to wait for the running tasks to terminate;
  - `Error()` to consult the termination reason.
- A ticker implementation that ticks on start and closes the channel on destruction.

## Installation

To install Periodic, use the following command:

```sh
go get github.com/parametalol/periodic
```

## Usage

See [[examples/tick_test.go]] for a usage example.
