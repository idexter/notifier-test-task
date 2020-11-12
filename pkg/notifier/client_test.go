package notifier

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifier_Notify(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		testSrv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		}))

		messages := generateTestMessages(10)

		notifier := New(testSrv.URL, nil)
		n, err := notifier.Notify(messages...)
		notifier.Wait()

		assert.NoError(t, err)
		assert.Equal(t, len(messages), n)
	})

	t.Run("Context canceled", func(t *testing.T) {
		messages := generateTestMessages(3)

		notifier := New("", nil)
		notifier.OnError(func(message []byte, err error) {
			var nErr *NotifyErr
			ok := errors.As(err, &nErr)
			assert.True(t, ok)
			assert.Equal(t, TypeContextCanceled, nErr.Type)
		})
		notifier.Stop()

		n, err := notifier.Notify(messages...)
		require.Error(t, err)
		assert.Equal(t, 3, n)

		notifier.Wait()
	})

	t.Run("Unable to create a request", func(t *testing.T) {
		msg := []byte("test message")

		notifier := New("%", nil) //nolint: staticcheck
		notifier.OnError(func(message []byte, err error) {
			var nErr *NotifyErr
			ok := errors.As(err, &nErr)
			assert.True(t, ok)
			assert.Equal(t, TypeSendError, nErr.Type)
			assert.Equal(t, msgSendErrorRequest, nErr.Message)
		})
		n, err := notifier.Notify(msg)
		notifier.Wait()

		require.NoError(t, err)
		assert.Equal(t, 1, n)
	})

	t.Run("Timeout", func(t *testing.T) {
		testSrv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			time.Sleep(time.Second)
			writer.WriteHeader(http.StatusOK)
		}))

		msg := []byte("test message")

		transport := getTestTransport()
		transport.ResponseHeaderTimeout = time.Millisecond
		notifier := create(testSrv.URL, nil, transport)
		notifier.OnError(func(message []byte, err error) {
			var nErr *NotifyErr
			ok := errors.As(err, &nErr)
			assert.True(t, ok)
			assert.Equal(t, TypeSendError, nErr.Type)
			assert.Equal(t, msgSendErrorClient, nErr.Message)
		})
		_, err := notifier.Notify(msg)
		notifier.Wait()

		require.NoError(t, err)
	})

	t.Run("Limiter error", func(t *testing.T) {
		testSrv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		}))

		msg := generateTestMessages(10)

		notifier := New(testSrv.URL, &ClientParams{
			MaxConcurrentWorkers: 100,
			MaxRequestRate:       time.Second,
			MaxRequestsPerRate:   1,
		})
		notifier.OnError(func(message []byte, err error) {
			var nErr *NotifyErr
			ok := errors.As(err, &nErr)
			assert.True(t, ok)
			assert.Equal(t, TypeSendError, nErr.Type)
			if string(message) != "msg 1" {
				assert.Equal(t, msgSendErrorRateLimiter, nErr.Message)
			}
		})
		_, err := notifier.Notify(msg...)
		time.Sleep(time.Second)
		notifier.Stop()
		notifier.Wait()

		require.NoError(t, err)
	})
}

func TestNotifier_DifferentLimits(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		transport := getTestTransport()
		transport.MaxIdleConnsPerHost = 0
		transport.MaxIdleConns = 0
		transport.MaxConnsPerHost = 0

		notifier := create("", nil, transport)

		assert.Equal(t, 100, cap(notifier.workersLimiter))
	})

	t.Run("MaxIdleConns", func(t *testing.T) {
		transport := getTestTransport()
		transport.MaxIdleConns = 50

		notifier := create("", nil, transport)

		assert.Equal(t, transport.MaxIdleConns, cap(notifier.workersLimiter))
	})

	t.Run("MaxIdleConnsPerHost", func(t *testing.T) {
		transport := getTestTransport()
		transport.MaxIdleConnsPerHost = 40

		notifier := create("", nil, transport)

		assert.Equal(t, transport.MaxIdleConnsPerHost, cap(notifier.workersLimiter))
	})

	t.Run("MaxConnsPerHost", func(t *testing.T) {
		transport := getTestTransport()
		transport.MaxConnsPerHost = 30

		notifier := create("", nil, transport)

		assert.Equal(t, transport.MaxConnsPerHost, cap(notifier.workersLimiter))
	})

	t.Run("Custom Params", func(t *testing.T) {
		notifier := New("", &ClientParams{
			MaxConcurrentWorkers: 0,
			MaxRequestRate:       0,
			MaxRequestsPerRate:   0,
		})

		assert.Equal(t, 1, cap(notifier.workersLimiter))
	})

	t.Run("Limit exceeded", func(t *testing.T) {
		testSrv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			time.Sleep(time.Second * 1)
			writer.WriteHeader(http.StatusOK)
		}))

		messages := generateTestMessages(1000)
		transport := getTestTransport()
		transport.MaxIdleConnsPerHost = 0
		transport.MaxIdleConns = 0
		transport.MaxConnsPerHost = 0

		notifier := create(testSrv.URL, nil, transport)
		n, err := notifier.Notify(messages...)

		assert.Error(t, err)
		assert.NotZero(t, n)
		notifier.Wait()
	})
}

func TestNotifier_Rlimit(t *testing.T) {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	require.NoError(t, err)
	DefaultParams.MaxConcurrentWorkers = rLimit.Cur + 1

	transport := getTestTransport()
	transport.MaxIdleConnsPerHost = 0
	transport.MaxIdleConns = 0
	transport.MaxConnsPerHost = 0

	notifier := create("", nil, transport)

	assert.Equal(t, int(rLimit.Cur), cap(notifier.workersLimiter))
}

// generateTestMessages generates messages array for testing purposes.
func generateTestMessages(limit int) [][]byte {
	messages := make([][]byte, limit)
	for i := 0; i < limit; i++ {
		messages[i] = []byte(fmt.Sprintf("msg %d", i))
	}
	return messages
}

// getTestTransport returns copy of http.DefaultTransport for testing purposes.
func getTestTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
