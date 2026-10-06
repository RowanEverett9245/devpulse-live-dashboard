package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/infrai-examples/devpulse-live-dashboard/internal/dashboard"
)

const canonicalCall = "infrai.metrics.batch"

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(time.Duration)
}

type APIError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    map[string]any `json:"-"`
	StatusCode int            `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 20 * time.Second},
		sleep:   time.Sleep,
	}
}

func (c *Client) CreateChannel(ctx context.Context, channel string) error {
	body := struct {
		Channel string `json:"channel"`
		Type    string `json:"type"`
	}{channel, "private"}
	return c.post(ctx, "/v1/realtime/channel/create", body, "channel:"+channel)
}

func (c *Client) Deliver(ctx context.Context, channel string, snapshot dashboard.Snapshot) error {
	operation := "developer-operation:" + snapshot.EventID
	metricsBody := struct {
		Points         []dashboard.Point `json:"points"`
		IdempotencyKey string            `json:"idempotency_key"`
	}{snapshot.Points, operation + ":metrics"}
	if err := c.post(ctx, "/v1/metrics/batch", metricsBody, operation+":metrics"); err != nil {
		return err
	}

	publishBody := struct {
		Channel   string             `json:"channel"`
		Event     string             `json:"event"`
		Data      dashboard.Snapshot `json:"data"`
		AccountID string             `json:"account_id"`
	}{channel, "developer.operation", snapshot, snapshot.Service}
	return c.post(ctx, "/v1/realtime/publish", publishBody, operation+":realtime")
}

func (c *Client) post(ctx context.Context, path string, body any, idempotencyKey string) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			apiErr := decodeAPIError(env.Error, resp.StatusCode)
			if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				c.sleep(retryDelay(resp.Header.Get("Retry-After"), attempt))
				continue
			}
			return apiErr
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport response: %s", resp.Status)
		}
		return nil
	}
	return fmt.Errorf("request retry budget exhausted")
}

func decodeAPIError(raw json.RawMessage, status int) *APIError {
	detail := map[string]any{}
	_ = json.Unmarshal(raw, &detail)
	code, _ := detail["code"].(string)
	message, _ := detail["message"].(string)
	return &APIError{Code: code, Message: message, Details: detail, StatusCode: status}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 && header != "" {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(250*(1<<attempt)) * time.Millisecond
}
