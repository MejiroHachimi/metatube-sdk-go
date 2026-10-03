package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/metatube-community/metatube-sdk-go/imageutil"
	"github.com/stretchr/testify/require"
)

func TestBoundedQueueCoalescesAndRemovesCancelledWork(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrent = 1
	cfg.ImageQueueSize = 1
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("webp"))
	}), cfg, nil, nil)
	first := g.startImage("a", httptest.NewRequest("GET", "/v1/images/thumb/FANZA/a", nil))
	<-started
	second := g.startImage("b", httptest.NewRequest("GET", "/v1/images/thumb/FANZA/b", nil))
	duplicate := g.startImage("b", httptest.NewRequest("GET", "/v1/images/thumb/FANZA/b", nil))
	require.Same(t, second, duplicate)
	require.Equal(t, 503, request(g, "/v1/images/thumb/FANZA/c").Code)
	g.leaveImage("b", second)
	require.NoError(t, second.ctx.Err(), "one client cannot cancel a shared job")
	g.leaveImage("b", duplicate)
	<-second.done
	require.Equal(t, 504, second.value.status)
	require.Equal(t, int32(1), calls.Load(), "cancelled queued job must not reach SDK")
	close(release)
	<-first.done
	g.leaveImage("a", first)
	require.Equal(t, 200, first.value.status)
}

func TestLargeImagesSerializeUntilEncodingCompletes(t *testing.T) {
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 20, 20))))
	cfg := testConfig()
	cfg.ImagePixelBudget = 100
	cfg.RequestTimeout = 2 * time.Second
	firstDecoded := make(chan struct{})
	secondStarted := make(chan struct{})
	release := make(chan struct{})
	var active, peak atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/images/thumb/FANZA/b" {
			close(secondStarted)
		}
		m, _, err := imageutil.DecodeLimitedContext(r.Context(), bytes.NewReader(data.Bytes()))
		if err != nil {
			w.WriteHeader(504)
			return
		}
		n := active.Add(1)
		if n > peak.Load() {
			peak.Store(n)
		}
		defer active.Add(-1)
		if r.URL.Path == "/v1/images/thumb/FANZA/a" {
			close(firstDecoded)
			<-release
		}
		w.Header().Set("Content-Type", "image/webp")
		if err := imageutil.EncodeToWebP(w, m); err != nil {
			t.Error(err)
		}
	}), cfg, nil, nil)
	a := make(chan *httptest.ResponseRecorder, 1)
	b := make(chan *httptest.ResponseRecorder, 1)
	go func() { a <- request(g, "/v1/images/thumb/FANZA/a") }()
	<-firstDecoded
	go func() { b <- request(g, "/v1/images/thumb/FANZA/b") }()
	<-secondStarted
	select {
	case <-b:
		t.Fatal("large images overlapped")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	require.Equal(t, 200, (<-a).Code)
	require.Equal(t, 200, (<-b).Code)
	require.Equal(t, int32(1), peak.Load())
}

func TestDisconnectedClientsAndDeadlinesKeepRunningSlot(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrent = 1
	cfg.RequestTimeout = 60 * time.Millisecond
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(started)
		<-release
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("webp"))
	}), cfg, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	requestA := httptest.NewRequest("GET", "/v1/images/thumb/FANZA/a", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() { g.ServeHTTP(httptest.NewRecorder(), requestA); close(done) }()
	<-started
	cancel()
	<-done
	require.Equal(t, 504, request(g, "/v1/images/thumb/FANZA/b").Code)
	require.Equal(t, int32(1), calls.Load(), "disconnect must not free an active processing slot")
	close(release)
}
