package deployer

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// WaitForHealthy polls the given host port and path until it returns a 200 OK
// or the context times out.
func WaitForHealthy(ctx context.Context, hostPort int, path string, timeout time.Duration) error {
	if path == "" {
		// If no health path is provided, we just assume it's healthy once it starts.
		return nil
	}

	// Ensure path starts with /
	if path[0] != '/' {
		path = "/" + path
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", hostPort, path)
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timed out after %s", timeout)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					return nil // Healthy!
				}
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second): // Poll interval
		}
	}
}
