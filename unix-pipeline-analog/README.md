# Concurrent Email Processing Pipeline in Go

A concurrent data-processing pipeline written in Go.

The project demonstrates how to build a streaming pipeline with goroutines and channels while handling several real-world concurrency constraints:

- parallel I/O-bound operations;
- deduplication of shared results;
- request batching;
- bounded concurrency;
- graceful channel lifecycle management;
- deterministic final aggregation.

The processing flow looks like this:

```text
Email
  ↓
SelectUsers
  ↓
SelectMessages
  ↓
CheckSpam
  ↓
CombineResults
  ↓
Output
```

Each stage runs concurrently and communicates with the next one through channels.

---

## Overview

The pipeline receives email addresses and processes them through several independent stages:

1. Resolve emails into users.
2. Deduplicate users by ID.
3. Fetch message IDs in optimized batches.
4. Check messages for spam with a concurrency limit.
5. Sort and format the final results.

The implementation is designed around streaming rather than stage-by-stage accumulation.

That means downstream stages can begin working as soon as upstream stages produce data.

```text
producer ─→ stage 1 ─→ stage 2 ─→ stage 3 ─→ consumer
```

---

## Key Features

### Streaming Pipeline

Pipeline stages are connected dynamically through channels.

Each stage has the following signature:

```go
type cmd func(in, out chan interface{})
```

For a stage with index `i`:

```text
channels[i] → cmds[i] → channels[i+1]
```

All stages run concurrently.

This allows values to move through the pipeline immediately instead of waiting for an entire stage to finish first.

---

### Parallel User Resolution

User lookup simulates a slow external service call.

Instead of resolving emails sequentially:

```text
email1 → wait
email2 → wait
email3 → wait
```

lookups are performed concurrently:

```text
email1 ─→ GetUser ─┐
email2 ─→ GetUser ─┼→ users
email3 ─→ GetUser ─┘
```

Some email addresses may be aliases for the same user.

Users are therefore deduplicated by `User.ID`.

A mutex protects the shared set of resolved IDs so that checking and inserting an ID behaves as a single atomic operation.

---

### Batched Message Fetching

The message service accepts up to two users per request.

Instead of:

```text
User1 → request
User2 → request
User3 → request
User4 → request
```

the pipeline groups users into batches:

```text
[User1, User2] → request
[User3, User4] → request
```

Each completed batch is processed concurrently.

Every worker receives its own batch copy, avoiding unnecessary shared mutable state between goroutines.

---

### Bounded Spam Checking

Spam checks can run concurrently, but the service rejects requests when concurrency exceeds its limit.

The pipeline uses a buffered channel as a semaphore:

```go
make(chan struct{}, HasSpamMaxAsyncRequests)
```

Acquiring a slot:

```go
semaphore <- struct{}{}
```

Releasing it:

```go
<-semaphore
```

With a limit of five:

```text
request 1 ─┐
request 2 ─┤
request 3 ─┤ running
request 4 ─┤
request 5 ─┘

request 6 → waits
```

As soon as one running request finishes, the next one can proceed.

This provides bounded concurrency without maintaining a separate worker pool.

---

### Deterministic Result Aggregation

Concurrent stages naturally produce results in a non-deterministic order.

The final stage collects all spam-check results and sorts them by:

1. spam messages first;
2. non-spam messages second;
3. message ID in ascending order within each group.

Example:

```text
true 221945221381252775
true 357347175551886490
true 1595319133252549342
false 26236336874602209
false 59892029605752939
```

---

## Architecture

The project separates concurrency responsibilities between pipeline orchestration and individual processing stages.

```text
                    ┌──────────────────┐
emails ────────────→│   SelectUsers    │
                    └────────┬─────────┘
                             │ User
                             ▼
                    ┌──────────────────┐
                    │ SelectMessages   │
                    └────────┬─────────┘
                             │ MsgID
                             ▼
                    ┌──────────────────┐
                    │    CheckSpam     │
                    └────────┬─────────┘
                             │ MsgData
                             ▼
                    ┌──────────────────┐
                    │ CombineResults   │
                    └────────┬─────────┘
                             │ string
                             ▼
                           output
```

### `RunPipeline`

`RunPipeline` is responsible for:

- creating channels between stages;
- launching every stage in its own goroutine;
- wiring stage inputs and outputs;
- closing output channels after stage completion;
- waiting for the entire pipeline to finish.

The pipeline owns channel lifecycle management.

Individual stages only consume from `in` and produce into `out`.

---

## Concurrency Patterns

This project uses several common Go concurrency patterns.

### Fan-out

Independent work is distributed across multiple goroutines.

Used for:

- user resolution;
- message fetching;
- spam checks.

---

### Fan-in

Results from multiple workers are written back into the same output channel.

---

### Semaphore

A buffered channel limits the number of simultaneous spam-service requests.

---

### WaitGroup

`sync.WaitGroup` is used both:

- at the pipeline level;
- inside stages that launch their own workers.

This prevents stages from returning while background goroutines can still write to their output channels.

---

### Mutex-Protected Deduplication

`sync.Mutex` protects a shared set of user IDs during concurrent user resolution.

Only the smallest necessary critical section is locked:

```text
check ID
+
insert ID
```

Potentially blocking channel writes happen outside the lock.

---

## Channel Lifecycle

Channel ownership is handled by the pipeline rather than by individual stages.

The lifecycle of a stage is:

```text
start stage
↓
process input
↓
stage returns
↓
close(stage output)
↓
next stage finishes reading
↓
next stage returns
```

This lets termination propagate naturally through the pipeline.

A stage never closes its input channel.

---

## Performance

The simulated external services intentionally introduce latency:

| Operation | Approximate latency |
|---|---:|
| `GetUser` | 1 second |
| `GetMessages` | 1 second |
| `HasSpam` | 100 ms |

A sequential implementation would spend most of its time waiting on independent operations.

The concurrent design overlaps those waits while still respecting service limits.

Conceptually:

```text
GetUser       ─────────────
GetMessages          ─────────────
HasSpam                     ───────
```

instead of:

```text
GetUser       ─────────────
             GetMessages   ─────────────
                           HasSpam       ───────
```

---

## Project Structure

```text
.
├── spammer.go
├── common.go
├── main_test.go
├── go.mod
└── README.md
```

### `spammer.go`

Contains the pipeline implementation:

```text
RunPipeline
SelectUsers
SelectMessages
CheckSpam
CombineResults
```

### `common.go`

Contains:

- domain types;
- simulated external services;
- concurrency limits;
- execution statistics.

### `main_test.go`

Contains integration and concurrency-focused tests covering:

- streaming pipeline behavior;
- user alias deduplication;
- concurrent pipelines;
- request batching;
- execution-time constraints;
- expected service call counts.

---

## Running the Project

Run all tests:

```bash
go test -v
```

Run the main integration test:

```bash
go test -run TestTotal -v
```

Run pipeline-specific tests:

```bash
go test -run TestPipeline -v
```

Run concurrent pipeline tests:

```bash
go test -run TestParallelPiplines -v
```

---

## Race Detection

The project can also be checked with Go's race detector:

```bash
go test -race -v
```

On Windows, the race detector requires CGO and a working C compiler.

Check the current CGO setting:

```bash
go env CGO_ENABLED
```

In Git Bash:

```bash
export CGO_ENABLED=1
```

---

## What This Project Demonstrates

The main focus of the project is practical concurrent system design in Go.

It covers:

```text
streaming pipelines
goroutine orchestration
channel synchronization
bounded concurrency
batch processing
shared-state protection
deduplication
fan-out / fan-in
graceful shutdown
deterministic aggregation
```

A key design principle throughout the project is to avoid unnecessary shared mutable state.

Where state must be shared, synchronization is kept as narrow as possible.

Where ownership can be transferred instead, each goroutine receives its own data.

---

## Tech Stack

- Go
- Goroutines
- Channels
- `sync.WaitGroup`
- `sync.Mutex`
- Buffered-channel semaphores
- Standard library sorting utilities

---

## Summary

This project implements a concurrent processing pipeline that combines multiple Go concurrency patterns in a single workflow.

Each stage uses the concurrency model that best matches its workload:

```text
SelectUsers
→ parallel I/O + synchronized deduplication

SelectMessages
→ batching + parallel batch processing

CheckSpam
→ bounded concurrency

CombineResults
→ deterministic aggregation

RunPipeline
→ stage orchestration and lifecycle management
```

The result is a fully streaming pipeline that processes independent operations concurrently while keeping resource usage controlled and predictable.