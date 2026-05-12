package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Default tuning constants. The retry budget is 3 attempts; backoff is
// exponential (`2^n * 200ms`, n incremented before sleep).
const (
	defaultMaxAttempts = 3
	defaultBaseBackoff = 200 * time.Millisecond
	defaultHTTPTimeout = 10 * time.Second
	maxRetryAfterCap   = 30 * time.Second
)

// HTTPDoer is the minimal interface a Client needs from an HTTP client.
// Tests substitute httptest.NewServer's default client to drive the retry
// table without leaving the process.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// SleepFunc is the seam tests use to assert backoff math without spending
// wall-clock time. Default is time.Sleep.
type SleepFunc func(d time.Duration)

// Client posts Messages to a Slack incoming webhook with retry/backoff
// semantics:
//   - 3 total attempts.
//   - Backoff = `2^numTries * baseBackoff`, numTries incremented before
//     the wait → first wait 400ms, second wait 800ms (with baseBackoff
//     = 200ms).
//   - 5xx / network errors / timeouts retry.
//   - 429 retries and honors Retry-After (seconds integer or HTTP date),
//     capped at 30s.
//   - 4xx (non-429) → terminal, returned wrapped.
type Client struct {
	webhookURL  string
	http        HTTPDoer
	maxAttempts int
	baseBackoff time.Duration
	sleep       SleepFunc
	now         func() time.Time
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the HTTP doer (the default is an http.Client with
// a 10s timeout).
func WithHTTPClient(h HTTPDoer) Option { return func(c *Client) { c.http = h } }

// WithMaxAttempts sets the total number of POST attempts (default 3).
func WithMaxAttempts(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxAttempts = n
		}
	}
}

// WithBaseBackoff sets the base unit for exponential backoff. Tests use a
// very small value (1ms) to keep the suite fast; production defaults to
// 200ms.
func WithBaseBackoff(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.baseBackoff = d
		}
	}
}

// WithSleep injects the sleep function used between retries — kept as a
// seam so tests can assert backoff calls without burning real time.
func WithSleep(s SleepFunc) Option { return func(c *Client) { c.sleep = s } }

// WithNow injects the clock used to parse HTTP-date Retry-After headers.
func WithNow(now func() time.Time) Option { return func(c *Client) { c.now = now } }

// New returns a Client targeting the given incoming-webhook URL.
//
// Defaults: 10s HTTP timeout, 3 attempts, 200ms base backoff, time.Sleep,
// time.Now. Override any of these with the matching Option.
func New(webhookURL string, opts ...Option) *Client {
	c := &Client{
		webhookURL:  webhookURL,
		http:        &http.Client{Timeout: defaultHTTPTimeout},
		maxAttempts: defaultMaxAttempts,
		baseBackoff: defaultBaseBackoff,
		sleep:       time.Sleep,
		now:         time.Now,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ErrTerminal wraps a 4xx (non-429) response — callers should not retry.
var ErrTerminal = errors.New("slack: terminal status from webhook")

// Post serializes m as JSON and POSTs it to the configured webhook URL,
// retrying per the retry contract documented on Client.
//
// Returns nil on first 2xx response. Returns a non-nil error on terminal
// failure so the Lambda handler can surface it as a non-nil return value
// (and the Lambda runtime increments the Errors metric).
func (c *Client) Post(ctx context.Context, m *Message) error {
	if c.webhookURL == "" {
		return errors.New("slack: webhook url is not configured")
	}
	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("slack: marshal message: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		if attempt > 0 {
			wait := c.backoffFor(attempt, lastErr)
			if wait > 0 {
				if werr := c.sleepCtx(ctx, wait); werr != nil {
					return werr
				}
			}
		}

		statusCode, retryHeader, perr := c.doOnce(ctx, body)
		if perr == nil {
			return nil
		}
		lastErr = perr

		var hr *httpError
		switch {
		case errors.As(perr, &hr) && hr.terminal:
			return fmt.Errorf("%w: %d", ErrTerminal, hr.status)
		case errors.As(perr, &hr) && hr.retryAfter != "":
			// 429: backoffFor will read retryHeader to compute the wait.
			_ = retryHeader
			_ = statusCode
		}
	}
	return fmt.Errorf("slack: exhausted %d attempts: %w", c.maxAttempts, lastErr)
}

// doOnce issues a single POST and classifies the response.
func (c *Client) doOnce(ctx context.Context, body []byte) (status int, retryAfter string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("slack: post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Drain the body so the underlying transport can reuse the connection.
	rawBody, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return resp.StatusCode, "", nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return resp.StatusCode, resp.Header.Get("Retry-After"), &httpError{
			status:     resp.StatusCode,
			retryAfter: resp.Header.Get("Retry-After"),
			body:       string(rawBody),
		}
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return resp.StatusCode, "", &httpError{
			status:   resp.StatusCode,
			terminal: true,
			body:     string(rawBody),
		}
	default:
		return resp.StatusCode, "", &httpError{
			status: resp.StatusCode,
			body:   string(rawBody),
		}
	}
}

// backoffFor returns the wait before the given attempt index (0-based,
// attempt==1 is "the second POST"). Honors 429 Retry-After when the last
// error carries one.
func (c *Client) backoffFor(attempt int, lastErr error) time.Duration {
	var hr *httpError
	if errors.As(lastErr, &hr) && hr.retryAfter != "" {
		if d, ok := parseRetryAfter(hr.retryAfter, c.now()); ok {
			if d > maxRetryAfterCap {
				d = maxRetryAfterCap
			}
			return d
		}
	}
	// `2^attempt * base` — attempt is the post-increment numTries.
	return time.Duration(1<<uint(attempt)) * c.baseBackoff
}

// sleepCtx sleeps for d unless the context expires first.
func (c *Client) sleepCtx(ctx context.Context, d time.Duration) error {
	if ctx.Err() != nil {
		return fmt.Errorf("slack: %w", ctx.Err())
	}
	type result struct{}
	done := make(chan result, 1)
	go func() {
		c.sleep(d)
		done <- result{}
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("slack: %w", ctx.Err())
	case <-done:
		return nil
	}
}

// httpError carries the metadata Post needs to decide whether to retry.
type httpError struct {
	status     int
	terminal   bool
	retryAfter string
	body       string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("slack: status %d: %s", e.status, e.body)
}

// parseRetryAfter accepts both forms RFC 7231 allows for Retry-After:
// an integer number of seconds or an HTTP date.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := t.Sub(now)
		if d < 0 {
			return 0, true
		}
		return d, true
	}
	return 0, false
}

// LogPost wraps Post for callers that want to record send failures without
// failing the entire batch. The handler uses this when multiple records
// fan out and one Slack call fails — the others still go out.
func (c *Client) LogPost(ctx context.Context, m *Message, logger *slog.Logger) error {
	if err := c.Post(ctx, m); err != nil {
		logger.ErrorContext(ctx, "slack post failed", "err", err)
		return err
	}
	return nil
}
