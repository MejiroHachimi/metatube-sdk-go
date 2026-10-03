package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func waitMetadataUsers(t *testing.T, g *Gateway, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		g.metadataMu.Lock()
		defer g.metadataMu.Unlock()
		n := 0
		for _, work := range g.metadata {
			n += work.users
		}
		return n == want
	}, 3*time.Second, time.Millisecond)
}

func TestMetadataCoalescesOnlyOverlappingRequests(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrent = 1
	cfg.RequestTimeout = 5 * time.Second
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		<-release
		fmt.Fprint(w, `{"data":{"fixture":true}}`)
	}), cfg, nil, nil)
	const clients = 32
	results := make(chan int, clients)
	for i := 0; i < clients; i++ {
		go func(i int) {
			path := "/v1/movies/search?q=fixture&fallback=false"
			if i%2 == 0 {
				path = "/v1/movies/search?fallback=false&q=fixture"
			}
			results <- request(g, path).Code
		}(i)
	}
	waitMetadataUsers(t, g, clients)
	require.Equal(t, int32(1), calls.Load())
	unblock()
	for i := 0; i < clients; i++ {
		require.Equal(t, http.StatusOK, <-results)
	}
	require.Equal(t, http.StatusOK, request(g, "/v1/movies/search?q=fixture&fallback=false").Code)
	require.Equal(t, int32(2), calls.Load(), "completed metadata must not be cached")
}

func TestMetadataAuthorizationIsolation(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrent = 2
	cfg.RequestTimeout = 5 * time.Second
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		<-release
		fmt.Fprint(w, `{"data":{"fixture":true}}`)
	}), cfg, nil, nil)
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			r := httptest.NewRequest("GET", "/v1/movies/FANZA/fixture", nil)
			r.Header.Set("Authorization", "Bearer fixture-token")
			w := httptest.NewRecorder()
			g.ServeHTTP(w, r)
			results <- w.Code
		}()
	}
	waitMetadataUsers(t, g, 2)
	require.Equal(t, http.StatusUnauthorized, request(g, "/v1/movies/FANZA/fixture").Code)
	require.Equal(t, int32(2), calls.Load(), "different credentials must not share a response")
	unblock()
	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusOK, <-results)
	}
}

func TestMetadataQueueCancellationAndSharedLifetime(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrent = 1
	cfg.MetadataQueueSize = 1
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		fmt.Fprint(w, `{"data":{}}`)
	}), cfg, nil, nil)
	firstKey := metadataKey{url: "first"}
	queuedKey := metadataKey{url: "queued"}
	r := httptest.NewRequest("GET", "/v1/movies/FANZA/fixture", nil)
	first := g.startMetadata(firstKey, r)
	<-started
	queued := g.startMetadata(queuedKey, r)
	duplicate := g.startMetadata(queuedKey, r)
	require.Same(t, queued, duplicate)
	require.Nil(t, g.startMetadata(metadataKey{url: "overflow"}, r))
	g.leaveMetadata(queuedKey, queued)
	require.NoError(t, queued.ctx.Err(), "one client cannot cancel the other client")
	g.leaveMetadata(queuedKey, duplicate)
	<-queued.done
	require.Equal(t, http.StatusGatewayTimeout, queued.value.status)
	require.Equal(t, int32(1), calls.Load(), "cancelled queued work must never reach the SDK")
	unblock()
	<-first.done
	g.leaveMetadata(firstKey, first)
	require.Empty(t, g.slots)
	require.Empty(t, g.metadataJobs)
}

func TestMetadataDisconnectDoesNotReleaseRunningSlot(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrent = 1
	cfg.MetadataQueueSize = 0
	cfg.RequestTimeout = 100 * time.Millisecond
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release // Emulate a provider that does not support cancellation.
		}
		fmt.Fprint(w, `{"data":{}}`)
	}), cfg, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest("GET", "/v1/movies/FANZA/fixture", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() { g.ServeHTTP(httptest.NewRecorder(), r); close(done) }()
	<-started
	cancel()
	<-done
	require.Len(t, g.slots, 1)
	require.Equal(t, http.StatusServiceUnavailable, request(g, "/v1/movies/FANZA/other").Code)
	// Images share the SDK bound and must not overlap the detached metadata task.
	require.Equal(t, http.StatusGatewayTimeout, request(g, "/v1/images/thumb/FANZA/other").Code)
	require.Equal(t, int32(1), calls.Load())
	unblock()
	require.Eventually(t, func() bool { return len(g.slots) == 0 && len(g.metadataJobs) == 0 }, time.Second, time.Millisecond)
	require.Equal(t, http.StatusOK, request(g, "/v1/movies/FANZA/other").Code)
}

func TestMetadataQueueDeadlineIncludesWaiting(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrent = 1
	cfg.MetadataQueueSize = 1
	cfg.RequestTimeout = 50 * time.Millisecond
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		fmt.Fprint(w, `{"data":{}}`)
	}), cfg, nil, nil)
	firstResult := make(chan int, 1)
	go func() { firstResult <- request(g, "/v1/movies/FANZA/first").Code }()
	<-started
	require.Equal(t, http.StatusGatewayTimeout, request(g, "/v1/movies/FANZA/queued").Code)
	require.Equal(t, http.StatusGatewayTimeout, <-firstResult)
	require.Equal(t, int32(1), calls.Load(), "expired queue entry must not start the SDK")
	require.Len(t, g.slots, 1, "HTTP timeout does not stop the running provider")
	unblock()
	require.Eventually(t, func() bool { return len(g.slots) == 0 && len(g.metadataJobs) == 0 }, time.Second, time.Millisecond)
}

func TestWaitingLimitKeepsHealthAndCacheAvailable(t *testing.T) {
	cfg := testConfig()
	cfg.MaxWaitingRequests = 16
	cfg.RequestTimeout = 5 * time.Second
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		<-release
		fmt.Fprint(w, `{"data":{}}`)
	}), cfg, nil, nil)
	results := make(chan int, cfg.MaxWaitingRequests)
	for i := 0; i < cfg.MaxWaitingRequests; i++ {
		go func() { results <- request(g, "/v1/movies/FANZA/fixture").Code }()
	}
	waitMetadataUsers(t, g, cfg.MaxWaitingRequests)
	for _, path := range []string{"/v1/movies/FANZA/fixture", "/v1/images/thumb/FANZA/uncached"} {
		w := request(g, path)
		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		require.Equal(t, "1", w.Header().Get("Retry-After"))
	}
	require.Equal(t, http.StatusOK, request(g, "/healthz").Code)
	require.Equal(t, http.StatusOK, request(g, "/readyz").Code)
	g.cache.put("/v1/images/thumb/FANZA/cached?", response{http.StatusOK, http.Header{"Content-Type": {"image/webp"}}, []byte("webp")})
	require.Equal(t, http.StatusOK, request(g, "/v1/images/thumb/FANZA/cached").Code)
	unblock()
	for i := 0; i < cfg.MaxWaitingRequests; i++ {
		require.Equal(t, http.StatusOK, <-results)
	}
	require.Empty(t, g.waiters)
	require.Equal(t, int32(1), calls.Load())
}

// A real HTTP burst with an entirely local, fixed-latency upstream. This checks
// admission and queueing; it is not a throughput estimate for external sites.
func TestMetadataHTTPBurst16(t *testing.T) {
	cfg := testConfig()
	cfg.MetadataQueueSize = 16
	cfg.RequestTimeout = 5 * time.Second
	var active, peak, calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		n := active.Add(1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(50 * time.Millisecond)
		fmt.Fprint(w, `{"data":{"fixture":true}}`)
	}), cfg, nil, nil)
	server := httptest.NewServer(g)
	defer server.Close()
	client := server.Client()
	client.Timeout = 6 * time.Second
	const count = 16
	type result struct {
		status  int
		elapsed time.Duration
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, count)
	for i := 0; i < count; i++ {
		go func(i int) {
			<-start
			begin := time.Now()
			r, err := client.Get(fmt.Sprintf("%s/v1/movies/FANZA/fixture%d", server.URL, i))
			if err != nil {
				results <- result{err: err}
				return
			}
			_, err = io.Copy(io.Discard, r.Body)
			r.Body.Close()
			results <- result{status: r.StatusCode, elapsed: time.Since(begin), err: err}
		}(i)
	}
	close(start)
	latencies := make([]time.Duration, 0, count)
	for i := 0; i < count; i++ {
		r := <-results
		require.NoError(t, r.err)
		require.Equal(t, http.StatusOK, r.status)
		latencies = append(latencies, r.elapsed)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	require.Equal(t, int32(count), calls.Load())
	require.Equal(t, int32(cfg.MaxConcurrent), peak.Load())
	require.Empty(t, g.waiters)
	t.Logf("16 distinct HTTP requests: 16 successful; peak SDK jobs=%d; p50=%s; max=%s", peak.Load(), latencies[7].Round(time.Millisecond), latencies[15].Round(time.Millisecond))
}
