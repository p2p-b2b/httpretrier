package httpretrier_test // Use _test package for examples

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/p2p-b2b/httpretrier"
)

// Example demonstrates using exponential backoff.
func Example() {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count <= 3 { // Fail first 3 times
			fmt.Printf("Server: Request %d -> 500 Internal Server Error\n", count)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			fmt.Printf("Server: Request %d -> 200 OK\n", count)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Success after backoff"))
		}
	}))
	defer server.Close()

	// Create a client with exponential backoff.
	// Base delay 5ms, max delay 50ms, max 4 retries.
	retryClient := httpretrier.NewClient(
		4,
		httpretrier.ExponentialBackoff(5*time.Millisecond, 50*time.Millisecond),
		nil,
	)

	fmt.Println("Client: Making request with exponential backoff...")
	resp, err := retryClient.Get(server.URL)
	if err != nil {
		fmt.Printf("Client: Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Client: Received response: Status=%s, Body='%s'\n", resp.Status, string(body))
	// Note: Duration will vary slightly, but should reflect increasing delays.
	fmt.Printf("Client: Total time approx > %dms (due to backoff)\n", (5 + 10 + 20)) // 5ms + 10ms + 20ms delays

	// Example Output (delays are approximate):
	// Client: Making request with exponential backoff...
	// Server: Request 1 -> 500 Internal Server Error
	// Server: Request 2 -> 500 Internal Server Error
	// Server: Request 3 -> 500 Internal Server Error
	// Server: Request 4 -> 200 OK
	// Client: Received response: Status=200 OK, Body='Success after backoff'
	// Client: Total time approx > 35ms (due to backoff)
}
