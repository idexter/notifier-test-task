// Package notifier provides HTTP notification library.
package notifier

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"
)

// ClientParams provides custom limits which you can set when create new Client instance.
type ClientParams struct {
	MaxConcurrentWorkers uint64
	MaxRequestRate       time.Duration
	MaxRequestsPerRate   int
}

// DefaultParams client parameters which is used by default.
// If DefaultParams is used MaxConcurrentWorkers can be less than 100.
// The library tries to optimize MaxConcurrentWorkers using calculateOptimalWorkersLimit function.
var DefaultParams = &ClientParams{
	MaxConcurrentWorkers: 100,
	MaxRequestRate:       10 * time.Second,
	MaxRequestsPerRate:   100,
}

// Client implements HTTP notifier.
// Use New function to create properly initialized instance.
type Client struct {
	url         string
	notifyError func(message []byte, err error)
	client      *http.Client

	ctx             context.Context
	cancel          context.CancelFunc
	workers         sync.WaitGroup
	workersLimiter  chan struct{}
	requestsLimiter *rate.Limiter
}

// New creates new Client instance with configured "URL" and provided ClientParams.
// If params is nil it will use DefaultParams.
func New(url string, params *ClientParams) *Client {
	return create(url, params, http.DefaultTransport)
}

// create creates new client instance. It also used for testing purposes to replace Transport.
func create(url string, params *ClientParams, transport http.RoundTripper) *Client {
	if params == nil {
		params = DefaultParams
		params.MaxConcurrentWorkers = calculateOptimalWorkersLimit(transport)
	}

	if params.MaxConcurrentWorkers == 0 {
		params.MaxConcurrentWorkers = 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	n := &Client{
		url:             url,
		notifyError:     func(message []byte, err error) {},
		client:          &http.Client{Transport: transport},
		ctx:             ctx,
		cancel:          cancel,
		workersLimiter:  make(chan struct{}, params.MaxConcurrentWorkers),
		requestsLimiter: rate.NewLimiter(rate.Every(params.MaxRequestRate), params.MaxRequestsPerRate),
	}
	return n
}

// calculateOptimalWorkersLimit calculates workers limit based on http.Transport parameters and syscall.Rlimit for more efficient resource usage.
func calculateOptimalWorkersLimit(transport http.RoundTripper) uint64 {
	var rLimit syscall.Rlimit
	var limit = DefaultParams.MaxConcurrentWorkers
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		if limit > rLimit.Cur {
			limit = rLimit.Cur
		}
	}

	if t, ok := transport.(*http.Transport); ok {
		if t.MaxConnsPerHost > 0 && uint64(t.MaxConnsPerHost) < limit {
			return uint64(t.MaxConnsPerHost)
		}
		if t.MaxIdleConnsPerHost > 0 && uint64(t.MaxIdleConnsPerHost) < limit {
			return uint64(t.MaxIdleConnsPerHost)
		}
		if t.MaxIdleConns > 0 && uint64(t.MaxIdleConns) < limit {
			return uint64(t.MaxIdleConns)
		}
	}
	return limit
}

// Notify schedules batch of messages to be sent to event-handling service.
// Every message will be sent to the server concurrently so call of this function is non-blocking.
//
// It return number of already scheduled messages. It return nil error if everything is fine.
// If something goes wrong it will return NotifyErr. You can figure out exact reason using this error.
//
// To handle messages efficient and avoid to exhaust all resources there are limits for maximum number of concurrent workers.
// Also there are rate limits for requests to the servers. All limits can be adjusted using ClientParams.
//
// If workers limit exceeded function will return NotifyErr with TypeWorkersLimitExceeded type.
// If notifier has been stopped using Stop call it will return NotifyErr with TypeContextCanceled type.
func (c *Client) Notify(messages ...[]byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		var i int
		for _, msg := range messages {
			e := &NotifyErr{
				Type:    TypeContextCanceled,
				Message: "Client context canceled",
				Err:     err,
			}
			c.notifyError(msg, e)
			i++
		}
		return i, err
	}

	var i int
	for _, nextMsg := range messages {
		select {
		case c.workersLimiter <- struct{}{}:
			c.workers.Add(1)
			go c.worker(nextMsg)
		default:
			return i, &NotifyErr{
				Type:    TypeWorkersLimitExceeded,
				Message: "Workers limit exceeded",
				Err:     nil,
			}
		}
		i++
	}

	return i, nil
}

// worker handles single message.
func (c *Client) worker(message []byte) {
	defer c.workers.Done()
	defer func() { <-c.workersLimiter }()
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, c.url, bytes.NewReader(message))
	if err != nil {
		e := &NotifyErr{
			Type:    TypeSendError,
			Message: msgSendErrorRequest,
			Err:     err,
		}
		c.notifyError(message, e)
		return
	}
	if err := c.requestsLimiter.Wait(c.ctx); err != nil {
		e := &NotifyErr{
			Type:    TypeSendError,
			Message: msgSendErrorRateLimiter,
			Err:     err,
		}
		c.notifyError(message, e)
		return
	}
	if _, err := c.client.Do(req); err != nil { //nolint: bodyclose
		e := &NotifyErr{
			Type:    TypeSendError,
			Message: msgSendErrorClient,
			Err:     err,
		}
		c.notifyError(message, e)
		return
	}
}

// OnError sets custom error handler which can be used to handle messages that has not been proceed.
// It will pass exact message on which error has happened and NotifyErr as an err argument.
func (c *Client) OnError(handler func(message []byte, err error)) {
	if handler != nil {
		c.notifyError = handler
	}
}

// Stop cancel scheduled tasks.
func (c *Client) Stop() {
	c.cancel()
}

// Wait blocks execution until all already scheduled workers finishes their work.
func (c *Client) Wait() {
	c.workers.Wait()
}
