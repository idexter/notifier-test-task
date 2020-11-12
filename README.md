# notifier-test-task

![golangci-lint](https://github.com/idexter/notifier-test-task/workflows/golangci-lint/badge.svg)
![build](https://github.com/idexter/notifier-test-task/workflows/build/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/idexter/notifier-test-task)](https://goreportcard.com/report/github.com/idexter/notifier-test-task)
[![codecov](https://codecov.io/gh/idexter/notifier-test-task/branch/master/graph/badge.svg)](https://codecov.io/gh/idexter/notifier-test-task)

This project consist Go test task solution for "Senior Go Backend Developer" position at some international company.

To get full description of the task see `docs/TASK_DESCRIPTION.md` file.

The layout of the project based on [golang-standards/project-layout](https://github.com/golang-standards/project-layout).
Hence, the library is stored at `pkg/notifier` directory, and the executable is stored in `cmd/notify` directory.

## Solution explanation

- [x] A client is configured with a URL to which notifications are sent. Done using `New(url string, params *ClientParams) *Client`.
- [x] It implements a function that takes messages and notifies about them. Done using `Notify(messages ...string) (int, error)` [1]
- [x] This operation should be non-blocking for the caller.
- [x] Allow the caller to handle notification failures in case any requests should fail. Done using `OnError(handler func(message []byte, err error))`.
- [x] Make sure to handle spikes in notification activity don’t overload the event-handling service or exhaust your file descriptors. [2]
- [x] But be efficient and don’t just send requests serially. [3]

#### Assumptions

- [1] As soon as there is no information about server in task description I assumed that it's ok do not check HTTP response codes like `200` or others.
As a library author I can't get assumptions about how exactly event-handling server will be implemented.

- [2] To prevent exhausting file descriptors I check `syscall.Rlimit` and set workers limit less than available descriptors.
At the same time there is no warranty that caller doesn't have some other files or connections already opened, so it's just sanity check.
Also, I check `http.Transport` limits and adjust workers limit to those limits if it's possible.
To prevent overloading of the event-handling service I use internal `rateLimiter`. At the same time I know nothing about the server.
Hence, I provided `ClientParams` which library user can use to adjust rate limits and workers limits.

- [3] Because `http.Client` is safe for concurrent calls I decided it will be efficient to handle each message in separate goroutine.
At the same time I decided to limit number of workers to avoid exhausting a lot of memory and CPU resources.
Caller can get an error from `Notify` and take care of it, if workers limit is exceeded.

## Requirements

You need `golangci-lint` installed locally to run linters with:
```
$ make lint
```

## Docs

Docs are available at `http://localhost:8888` after you start `$ make docs`.
It will also show you link to the package in the console output to faster navigation.

## Build

Just run:
```
$ make build
```

It will create `notify` binary inside the project root which is `The Executable` application from `docs/TASK_DESCRIPTION.md`.
Also, it will create `notify-test-server` which is simple http server created for testing purposes.

## Test

All tests can be started using:
```
$ make test
```

You also can get coverage report using:
```
$ make coverage
```
