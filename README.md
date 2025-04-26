# httpretrier

[![Go Reference](https://pkg.go.dev/badge/github.com/p2p-b2b/httpretrier.svg)](https://pkg.go.dev/github.com/p2p-b2b/httpretrier)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/p2p-b2b/httpretrier?style=plastic)

`httpretrier` is a Go library that provides a convenient way to add automatic retry logic to your HTTP requests. It wraps the standard `http.Client` and `http.Transport` to handle transient server errors (5xx) or network issues by retrying requests based on configurable strategies.

## Features

* **Automatic Retries:** Automatically retries requests that fail due to server errors (5xx) or transport-level errors.
* **Configurable Retry Strategies:**
  * `FixedDelay`: Retries after a constant delay.
  * `ExponentialBackoff`: Retries with exponentially increasing delays.
  * `JitterBackoff`: Retries with exponential backoff plus random jitter to prevent thundering herd issues.
* **Flexible Configuration:** Use the `ClientBuilder` for fine-grained control over:
  * Maximum number of retries.
  * Base and maximum delay for backoff strategies.
  * Standard `http.Transport` settings (timeouts, keep-alives, connection pooling).
  * Overall request timeout (`http.Client.Timeout`).
* **Easy Integration:** Designed as a drop-in replacement for `http.Client`.

## Installation

```bash
go get github.com/p2p-b2b/httpretrier
```

## Usage

### Basic Usage with Default Transport

You can quickly create a client with a specific retry strategy and number of retries using `httpretrier.NewClient`. It uses `http.DefaultTransport` underneath.

```go
package main

import (
  "fmt"
  "io"
  "net/http"
  "net/http/httptest"
  "sync/atomic"
  "time"

  "github.com/p2p-b2b/httpretrier"
)

func main() {
  var requestCount int32
  // Example server that fails the first few requests
  server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    count := atomic.AddInt32(&requestCount, 1)
    if count <= 2 { // Fail first 2 times
      fmt.Printf("Server: Request %d -> 500 Internal Server Error\n", count)
      w.WriteHeader(http.StatusInternalServerError)
    } else {
      fmt.Printf("Server: Request %d -> 200 OK\n", count)
      w.WriteHeader(http.StatusOK)
      _, _ = w.Write([]byte("Success!"))
    }
  }))
  defer server.Close()

  // Create a client with exponential backoff (3 retries, 10ms base, 100ms max delay)
  retryClient := httpretrier.NewClient(
    3, // Max Retries
    httpretrier.ExponentialBackoff(10*time.Millisecond, 100*time.Millisecond),
    nil, // Use http.DefaultTransport
  )

  fmt.Println("Client: Making request...")
  resp, err := retryClient.Get(server.URL)
  if err != nil {
    fmt.Printf("Client: Request failed after retries: %v\n", err)
    return
  }
  defer resp.Body.Close()

  body, _ := io.ReadAll(resp.Body)
  fmt.Printf("Client: Received response: Status=%s, Body='%s'\n", resp.Status, string(body))
}

// Example Output:
// Client: Making request...
// Server: Request 1 -> 500 Internal Server Error
// Server: Request 2 -> 500 Internal Server Error
// Server: Request 3 -> 200 OK
// Client: Received response: Status=200 OK, Body='Success!'
```

### Advanced Configuration with ClientBuilder

For more control over the client and transport settings, use the `ClientBuilder`.

```go
package main

import (
  "fmt"
  "io"
  "net/http"
  "net/http/httptest"
  "time"

  "github.com/p2p-b2b/httpretrier"
)

func main() {
  // Example server
  server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("Builder success!"))
  }))
  defer server.Close()

  // Use the builder for detailed configuration
  builder := httpretrier.NewClientBuilder()

  httpClient := builder.
    WithTimeout(15 * time.Second).          // Overall request timeout
    WithMaxRetries(5).                      // Max 5 retries
    WithRetryStrategy(httpretrier.JitterBackoffStrategy). // Use Jitter strategy
    WithRetryBaseDelay(100 * time.Millisecond). // 100ms base delay
    WithRetryMaxDelay(2 * time.Second).       // 2s max delay
    WithMaxIdleConns(50).                   // Transport: Max 50 idle connections
    WithIdleConnTimeout(30 * time.Second).    // Transport: 30s idle timeout
    Build()                                 // Build the http.Client

  fmt.Println("Client (Builder): Making request...")
  resp, err := httpClient.Get(server.URL)
  if err != nil {
    fmt.Printf("Client (Builder): Request failed: %v\n", err)
    return
  }
  defer resp.Body.Close()

  body, _ := io.ReadAll(resp.Body)
  fmt.Printf("Client (Builder): Received response: Status=%s, Body='%s'\n", resp.Status, string(body))
}

// Example Output:
// Client (Builder): Making request...
// Client (Builder): Received response: Status=200 OK, Body='Builder success!'
```

## Configuration Options (ClientBuilder)

The `ClientBuilder` allows configuration of:

* **Retry Logic:**
  * `WithMaxRetries(int)`: Maximum number of retry attempts.
  * `WithRetryStrategy(httpretrier.Strategy)`: Set the strategy (`FixedDelayStrategy`, `ExponentialBackoffStrategy`, `JitterBackoffStrategy`).
  * `WithRetryBaseDelay(time.Duration)`: Base delay for backoff/jitter, or the fixed delay duration.
  * `WithRetryMaxDelay(time.Duration)`: Maximum delay cap for backoff/jitter strategies.
* **HTTP Client:**
  * `WithTimeout(time.Duration)`: Sets the `Timeout` field on the resulting `http.Client`.
* **HTTP Transport:** (Controls the underlying `http.Transport`)
  * `WithMaxIdleConns(int)`
  * `WithIdleConnTimeout(time.Duration)`
  * `WithTLSHandshakeTimeout(time.Duration)`
  * `WithExpectContinueTimeout(time.Duration)`
  * `WithDisableKeepAlives(bool)`
  * `WithMaxIdleConnsPerHost(int)`

See the Go documentation for default values and validation ranges for these parameters.

## License

This library is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
