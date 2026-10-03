package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{CacheBytes: 1 << 20, CacheTTL: time.Hour, RequestTimeout: time.Second, MaxConcurrent: 4, MaxWaitingRequests: 128, MetadataQueueSize: 16, ImageQueueSize: 16, ImagePixelBudget: 6_000_000, ExcludedGenres: strings.Split(DefaultExcludedGenres, ",")}
}
func request(h http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}
func TestCleanMetadata(t *testing.T) {
	raw := `{"data":{"title":"<b>A &amp; B</b>","summary":"<script>alert(1)</script><p>Plain</p>","genres":["1080P"," Blu-ray ","蓝光","Drama","drama","Not 1080P",""],"actors":null,"preview_images":["https://cdn.example/a?x=1&y=2"]}}`
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, raw) }), testConfig(), nil, nil)
	w := request(g, "/v1/movies/FANZA/id")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var got struct {
		Data struct {
			Title, Summary string
			Genres, Actors []string
			Images         []string `json:"preview_images"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Data.Title != "A & B" || got.Data.Summary != "Plain" {
		t.Fatalf("text not clean: %+v", got.Data)
	}
	if strings.Join(got.Data.Genres, ",") != "Drama,Not 1080P" || got.Data.Actors == nil {
		t.Fatalf("lists not clean: %+v", got.Data)
	}
	if got.Data.Images[0] != "https://cdn.example/a?x=1&y=2" {
		t.Fatal("URL altered")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("metadata cache policy")
	}
}
func TestImageCacheVariantsAndConditionalRequests(t *testing.T) {
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "image/webp")
		fmt.Fprint(w, "webp:", r.URL.RawQuery)
	}), testConfig(), nil, nil)
	path := "/v1/images/primary/FANZA/id?quality=90&pos=1"
	first := request(g, path)
	second := request(g, "/v1/images/primary/FANZA/id?pos=1&quality=90")
	if calls.Load() != 1 || first.Header().Get("X-Cache") != "MISS" || second.Header().Get("X-Cache") != "HIT" {
		t.Fatal("cache failed", calls.Load())
	}
	request(g, "/v1/images/primary/FANZA/id?pos=0&quality=90")
	if calls.Load() != 2 {
		t.Fatal("different crop shared cache")
	}
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("If-None-Match", `"other", W/`+first.Header().Get("ETag"))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	if w.Code != 304 || w.Body.Len() != 0 {
		t.Fatal("conditional request", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("HEAD", path, nil)
	w = httptest.NewRecorder()
	g.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") == "0" {
		t.Fatal("HEAD failed")
	}
}
func TestErrorsNeverCachedOrLeaked(t *testing.T) {
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=9999")
		w.WriteHeader(500)
		fmt.Fprint(w, "secret and private upstream URL")
	}), testConfig(), nil, nil)
	for range 2 {
		w := request(g, "/v1/images/thumb/FANZA/id")
		if w.Code != 500 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "secret") {
			t.Fatal(w)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("cached failure")
	}
}
func TestConcurrentImagesCoalesce(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		w.Header().Set("Content-Type", "image/webp")
		fmt.Fprint(w, "webp")
	}), testConfig(), nil, nil)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request(g, "/v1/images/thumb/FANZA/id")
			if w.Code != 200 {
				t.Errorf("status %d", w.Code)
			}
		}()
	}
	<-started
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate fetches", calls.Load())
	}
}
func TestValidation(t *testing.T) {
	var calls atomic.Int32
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, `{"data":{}}`) }), testConfig(), nil, nil)
	for _, path := range []string{"/v1/movies/search", "/v1/movies/search?q=x&q=y", "/v1/movies/search?q=x&unexpected=y", "/v1/images/primary/FANZA/id?ratio=NaN", "/v1/images/primary/FANZA/id?pos=2", "/v1/images/primary/FANZA/id?quality=101", "/v1/images/primary/FANZA/id?badge=https://example.org/x", "/v1/movies/FANZA/id?lazy=maybe"} {
		if w := request(g, path); w.Code != 400 {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input reached SDK")
	}
	if w := request(g, "/unknown"); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestTimeoutAndConcurrencyBound(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	cfg := testConfig()
	cfg.MaxConcurrent = 1
	cfg.MetadataQueueSize = 0
	cfg.RequestTimeout = 20 * time.Millisecond
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; fmt.Fprint(w, `{"data":{}}`) }), cfg, nil, nil)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- request(g, "/v1/movies/FANZA/id") }()
	<-started
	busy := request(g, "/v1/movies/FANZA/other")
	if busy.Code != 503 {
		t.Fatal("limit not enforced", busy.Code)
	}
	w := <-done
	if w.Code != 504 || !json.Valid(w.Body.Bytes()) {
		t.Fatal("timeout not JSON", w.Code)
	}
	close(release)
}
func TestReadinessAndDocs(t *testing.T) {
	g := NewGateway(http.NotFoundHandler(), testConfig(), nil, func(context.Context) error { return fmt.Errorf("private database secret") })
	if w := request(g, "/healthz"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := request(g, "/readyz"); w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w)
	}
	if w := request(g, "/openapi.json"); w.Code != 200 || !json.Valid(w.Body.Bytes()) {
		t.Fatal("invalid spec")
	}
	if w := request(g, "/docs"); w.Code != 200 || strings.Contains(w.Body.String(), "cdn.jsdelivr") {
		t.Fatal("docs not self contained")
	}
}
func TestCacheEvictionExpiryAndOversize(t *testing.T) {
	now := time.Now()
	c := newImageCache(600, time.Minute)
	c.now = func() time.Time { return now }
	value := response{200, make(http.Header), make([]byte, 200)}
	c.put("a", value)
	c.put("b", value)
	if _, ok := c.get("a"); ok {
		t.Fatal("LRU did not evict")
	}
	if _, ok := c.get("b"); !ok {
		t.Fatal("new entry missing")
	}
	now = now.Add(time.Minute)
	if _, ok := c.get("b"); ok {
		t.Fatal("expired entry returned")
	}
	c.put("huge", response{body: make([]byte, 700)})
	if c.bytes != 0 {
		t.Fatal("oversize entry retained")
	}
}
