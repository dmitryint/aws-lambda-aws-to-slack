package slack

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestPost_Success(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, WithBaseBackoff(time.Millisecond), WithSleep(func(time.Duration) {}))
	if err := c.Post(t.Context(), NewMessage("good", "ok")); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d, want 1", got)
	}
}

func TestPost_RetriesOn5xxAndExhausts(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var sleeps []time.Duration
	c := New(srv.URL,
		WithBaseBackoff(time.Millisecond),
		WithSleep(func(d time.Duration) { sleeps = append(sleeps, d) }),
	)
	err := c.Post(t.Context(), NewMessage("danger", "boom"))
	if err == nil {
		t.Fatalf("expected error after exhausting retries")
	}
	if got := calls.Load(); got != defaultMaxAttempts {
		t.Fatalf("calls = %d, want %d", got, defaultMaxAttempts)
	}
	// 3 attempts → 2 sleeps. With baseBackoff=1ms and exponential
	// backoff `2^n * base`, waits should be 2ms and 4ms (n=1, n=2).
	if len(sleeps) != 2 {
		t.Fatalf("sleeps = %d, want 2", len(sleeps))
	}
	if sleeps[0] != 2*time.Millisecond || sleeps[1] != 4*time.Millisecond {
		t.Fatalf("backoff math wrong: %v", sleeps)
	}
}

func TestPost_RetriesOn429HonoringRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var sleeps []time.Duration
	c := New(srv.URL,
		WithBaseBackoff(time.Millisecond),
		WithSleep(func(d time.Duration) { sleeps = append(sleeps, d) }),
	)
	if err := c.Post(t.Context(), NewMessage("warning", "wait")); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
	if len(sleeps) != 1 {
		t.Fatalf("sleeps = %d, want 1", len(sleeps))
	}
	// Retry-After: 1 → 1s, ignoring the otherwise-2ms backoff.
	if sleeps[0] != 1*time.Second {
		t.Fatalf("Retry-After ignored: %v", sleeps[0])
	}
}

func TestPost_429RetryAfterHTTPDate(t *testing.T) {
	var calls atomic.Int32
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	retryAt := now.Add(2 * time.Second).UTC().Format(http.TimeFormat)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", retryAt)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var sleeps []time.Duration
	c := New(srv.URL,
		WithBaseBackoff(time.Millisecond),
		WithSleep(func(d time.Duration) { sleeps = append(sleeps, d) }),
		WithNow(func() time.Time { return now }),
	)
	if err := c.Post(t.Context(), NewMessage("warning", "wait")); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if len(sleeps) != 1 || sleeps[0] != 2*time.Second {
		t.Fatalf("HTTP-date Retry-After ignored: %v", sleeps)
	}
}

func TestPost_429RetryAfterPastDateClampsToZero(t *testing.T) {
	var calls atomic.Int32
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	retryAt := now.Add(-1 * time.Hour).UTC().Format(http.TimeFormat)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", retryAt)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var sleeps []time.Duration
	c := New(srv.URL,
		WithBaseBackoff(time.Millisecond),
		WithSleep(func(d time.Duration) { sleeps = append(sleeps, d) }),
		WithNow(func() time.Time { return now }),
	)
	if err := c.Post(t.Context(), NewMessage("warning", "wait")); err != nil {
		t.Fatalf("Post: %v", err)
	}
	// Past Retry-After produces a 0-duration wait; the sleep is skipped
	// entirely so the retry fires immediately. Two calls happened, no
	// sleeps recorded.
	if len(sleeps) != 0 {
		t.Fatalf("past Retry-After should skip sleep entirely: %v", sleeps)
	}
}

func TestPost_429RetryAfterCappedAt30s(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var sleeps []time.Duration
	c := New(srv.URL,
		WithBaseBackoff(time.Millisecond),
		WithSleep(func(d time.Duration) { sleeps = append(sleeps, d) }),
	)
	if err := c.Post(t.Context(), NewMessage("warning", "wait")); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if len(sleeps) != 1 || sleeps[0] != maxRetryAfterCap {
		t.Fatalf("Retry-After cap not applied: %v", sleeps)
	}
}

func TestPost_4xxNonRetryable(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := New(srv.URL, WithBaseBackoff(time.Millisecond), WithSleep(func(time.Duration) {}))
	err := c.Post(t.Context(), NewMessage("good", "ok"))
	if err == nil {
		t.Fatal("expected error on 400")
	}
	if !errors.Is(err, ErrTerminal) {
		t.Fatalf("expected ErrTerminal, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 4xx)", got)
	}
}

func TestPost_NetworkErrorCountsAsRetry(t *testing.T) {
	c := New("http://127.0.0.1:1", // closed port → connection refused
		WithBaseBackoff(time.Millisecond),
		WithSleep(func(time.Duration) {}),
	)
	err := c.Post(t.Context(), NewMessage("good", "ok"))
	if err == nil {
		t.Fatal("expected error on unreachable host")
	}
	// 3 attempts exhausted; the error is the wrapped final attempt error.
	if errors.Is(err, ErrTerminal) {
		t.Fatalf("network error should not be terminal: %v", err)
	}
}

func TestPost_MissingWebhookURL(t *testing.T) {
	c := New("")
	if err := c.Post(t.Context(), NewMessage("good", "ok")); err == nil {
		t.Fatal("expected error for empty webhook URL")
	}
}

func TestPost_ContextCancellationDuringBackoff(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	c := New(srv.URL,
		WithBaseBackoff(50*time.Millisecond),
		WithSleep(func(d time.Duration) { time.Sleep(d) }),
	)
	go func() {
		// Let the first POST land, then cancel during the backoff sleep.
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err := c.Post(ctx, NewMessage("good", "ok"))
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestPost_NonRetryAfter429ContinuesExponentialBackoff(t *testing.T) {
	// No Retry-After header → fallback to exponential backoff math.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var sleeps []time.Duration
	c := New(srv.URL,
		WithBaseBackoff(time.Millisecond),
		WithSleep(func(d time.Duration) { sleeps = append(sleeps, d) }),
	)
	if err := c.Post(t.Context(), NewMessage("warning", "wait")); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if len(sleeps) != 1 || sleeps[0] != 2*time.Millisecond {
		t.Fatalf("expected exponential fallback, got %v", sleeps)
	}
}

func TestPost_5xxSentBodyContainsRequest(t *testing.T) {
	// Sanity check the request body is the JSON-encoded message.
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		got = string(buf)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := New(srv.URL, WithBaseBackoff(time.Millisecond), WithSleep(func(time.Duration) {}))
	msg := NewMessage("danger", "fb", SectionBlock("hi"))
	if err := c.Post(t.Context(), msg); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if !contains(got, `"color":"danger"`) || !contains(got, `"text":"hi"`) {
		t.Fatalf("body missing expected fields: %s", got)
	}
}

// recordingDoer is a tiny seam to assert WithHTTPClient threading.
type recordingDoer struct {
	calls int
	resp  *http.Response
	err   error
}

func (r *recordingDoer) Do(*http.Request) (*http.Response, error) {
	r.calls++
	return r.resp, r.err
}

func TestWithHTTPClient_InjectsDoer(t *testing.T) {
	doer := &recordingDoer{err: errors.New("network down")}
	c := New("https://example/", WithHTTPClient(doer),
		WithBaseBackoff(time.Millisecond), WithSleep(func(time.Duration) {}))
	_ = c.Post(t.Context(), NewMessage("good", "ok"))
	if doer.calls != defaultMaxAttempts {
		t.Fatalf("doer.calls = %d, want %d", doer.calls, defaultMaxAttempts)
	}
}

func TestWithMaxAttempts_AndIgnoresNonPositive(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL,
		WithMaxAttempts(2),
		WithMaxAttempts(0), // ignored — must keep 2
		WithBaseBackoff(time.Millisecond),
		WithSleep(func(time.Duration) {}),
	)
	_ = c.Post(t.Context(), NewMessage("good", "ok"))
	if got := calls.Load(); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestWithBaseBackoff_IgnoresNonPositive(t *testing.T) {
	c := New("https://example/", WithBaseBackoff(0))
	if c.baseBackoff != defaultBaseBackoff {
		t.Fatalf("baseBackoff = %v, want default", c.baseBackoff)
	}
}

func TestLogPost_LogsAndReturnsError(t *testing.T) {
	c := New("") // missing URL → terminal error
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := c.LogPost(t.Context(), NewMessage("good", "ok"), logger); err == nil {
		t.Fatal("expected error")
	}
}

func TestLogPost_SuccessReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := New(srv.URL, WithBaseBackoff(time.Millisecond), WithSleep(func(time.Duration) {}))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := c.LogPost(t.Context(), NewMessage("good", "ok"), logger); err != nil {
		t.Fatalf("LogPost: %v", err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in       string
		wantOK   bool
		wantSecs int
	}{
		{in: "", wantOK: false},
		{in: "abc", wantOK: false},
		{in: "0", wantOK: true, wantSecs: 0},
		{in: "5", wantOK: true, wantSecs: 5},
		{in: now.Add(3 * time.Second).UTC().Format(http.TimeFormat), wantOK: true, wantSecs: 3},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			d, ok := parseRetryAfter(tc.in, now)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && d != time.Duration(tc.wantSecs)*time.Second {
				t.Fatalf("d = %v, want %v", d, time.Duration(tc.wantSecs)*time.Second)
			}
		})
	}
}

// ensure compile-time imports stay referenced when strconv is removed by
// future edits.
var _ = strconv.Atoi

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
