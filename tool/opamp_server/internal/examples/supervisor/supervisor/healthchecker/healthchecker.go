package healthchecker

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type HttpHealthChecker struct {
	endpoint string
}

func NewHttpHealthChecker(endpoint string) *HttpHealthChecker {
	return &HttpHealthChecker{
		endpoint: endpoint,
	}
}

func (h *HttpHealthChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", h.endpoint, nil)
	if err != nil {
		return err
	}

	client := http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check on %s returned %d", h.endpoint, resp.StatusCode)
	}

	return nil
}
