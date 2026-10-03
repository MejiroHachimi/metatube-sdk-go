package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func clearConfig(t *testing.T) {
	t.Helper()
	for _, key := range []string{"DATA_DIR", "DSN", "DATABASE_URL", "TOKEN", "PORT", "BIND", "IMAGE_CACHE_TTL", "REQUEST_TIMEOUT", "IMAGE_CACHE_MB", "MAX_CONCURRENT"} {
		t.Setenv(key, "")
	}
}

func TestConfigDefaultsOverridesAndInvalidValues(t *testing.T) {
	clearConfig(t)
	t.Setenv("EXCLUDED_GENRES", "")
	require.NoError(t, os.Unsetenv("EXCLUDED_GENRES"))
	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.Address)
	require.Equal(t, int64(64<<20), cfg.CacheBytes)
	require.Contains(t, cfg.ExcludedGenres, "1080p")
	for _, tc := range []struct{ key, value string }{
		{"PORT", "0"}, {"PORT", "abc"}, {"IMAGE_CACHE_MB", "1025"}, {"IMAGE_CACHE_MB", "abc"},
		{"IMAGE_CACHE_TTL", "0s"}, {"IMAGE_CACHE_TTL", "721h"}, {"REQUEST_TIMEOUT", "invalid"},
		{"MAX_CONCURRENT", "0"}, {"MAX_CONCURRENT", "65"}, {"MAX_CONCURRENT", "abc"},
		{"DATABASE_URL", "postgres://private/db"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			_, err := LoadConfig()
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private")
		})
	}
	for k, v := range map[string]string{"DATA_DIR": "/tmp/metatube", "DSN": "file:test.db", "BIND": "127.0.0.1", "PORT": "9090", "TOKEN": "example", "IMAGE_CACHE_MB": "0", "IMAGE_CACHE_TTL": "2h", "REQUEST_TIMEOUT": "3s", "MAX_CONCURRENT": "2", "EXCLUDED_GENRES": "4K,720p"} {
		t.Setenv(k, v)
	}
	cfg, err = LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:9090", cfg.Address)
	require.Equal(t, "file:test.db", cfg.DSN)
	require.Equal(t, "/tmp/metatube", cfg.DataDir)
	require.Equal(t, "example", cfg.Token)
	require.Zero(t, cfg.CacheBytes)
	require.Equal(t, 2*time.Hour, cfg.CacheTTL)
	require.Equal(t, 3*time.Second, cfg.RequestTimeout)
	require.Equal(t, 2, cfg.MaxConcurrent)
	require.Equal(t, []string{"4K", "720p"}, cfg.ExcludedGenres)
}

func TestStartupFailures(t *testing.T) {
	cfg := testConfig()
	cfg.MaxConcurrent = 0
	_, _, err := Open(cfg)
	require.Error(t, err)
	cfg = testConfig()
	cfg.DataDir = filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(cfg.DataDir, []byte("not a directory"), 0600))
	_, _, err = Open(cfg)
	require.Error(t, err)
	cfg.DSN = "postgres://secret/db"
	_, _, err = Open(cfg)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret")
}

func TestGatewayRejectsBadUpstreamResponses(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		status     int
		next       http.HandlerFunc
	}{
		{"invalid-json", "/v1/movies/FANZA/id", 502, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "invalid") }},
		{"wrong-image", "/v1/images/thumb/FANZA/id", 502, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			fmt.Fprint(w, "jpeg")
		}},
		{"redirect", "/v1/movies/FANZA/id", 502, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(302) }},
		{"oversize", "/v1/images/thumb/FANZA/id", 502, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(bytes.Repeat([]byte("x"), maxResponseBytes+1))
		}},
		{"panic", "/v1/movies/FANZA/id", 500, func(http.ResponseWriter, *http.Request) { panic("private secret") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGateway(tc.next, testConfig(), nil, nil)
			w := request(g, tc.path)
			require.Equal(t, tc.status, w.Code)
			require.True(t, json.Valid(w.Body.Bytes()))
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			require.NotContains(t, w.Body.String(), "secret")
		})
	}
}

func TestRequestMethodsAndParameterBoundaries(t *testing.T) {
	g := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"data":{}}`) }), testConfig(), nil, nil)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"POST", "/v1/providers", 405}, {"HEAD", "/v1/providers", 405},
		{"HEAD", "/healthz", 200}, {"HEAD", "/docs", 200}, {"HEAD", "/openapi.json", 200},
		{"GET", "/v1/images/unknown/FANZA/id", 400},
		{"GET", "/v1/images/thumb/FANZA/id?quality=abc", 400},
		{"GET", "/v1/movies/search?q=%zz", 400},
		{"GET", "/v1/movies/search?q=" + strings.Repeat("x", 8193), 400},
	} {
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, tc.status, w.Code)
		if tc.method == "HEAD" {
			require.Empty(t, w.Body.Bytes())
		}
	}
	for _, query := range []string{"", "?quality=1", "?quality=80", "?quality=100", "?quality="} {
		r := httptest.NewRequest("GET", "/v1/images/thumb/FANZA/id"+query, nil)
		image, err := validate(r)
		require.NoError(t, err)
		require.True(t, image)
		require.Empty(t, r.URL.RawQuery, "fixed quality must share one cache entry")
	}
	r := httptest.NewRequest("GET", "/v1/movies/FANZA/id?lazy=1", nil)
	_, err := validate(r)
	require.NoError(t, err)
	require.Equal(t, "lazy=true", r.URL.RawQuery)
}

func TestCacheFixedTTLReplacementAndDisabledStorage(t *testing.T) {
	now := time.Now()
	c := newImageCache(4096, time.Minute)
	c.now = func() time.Time { return now }
	c.put("a", response{body: []byte("old")})
	c.put("a", response{body: []byte("new")})
	require.Equal(t, int64(260), c.bytes)
	now = now.Add(30 * time.Second)
	v, ok := c.get("a")
	require.True(t, ok)
	require.Equal(t, "new", string(v.body))
	now = now.Add(30 * time.Second)
	c.put("b", response{body: []byte("new")})
	_, ok = c.get("a")
	require.False(t, ok, "reading must not extend TTL")
	require.Len(t, c.entries, 1)
	disabled := newImageCache(0, time.Hour)
	disabled.put("x", response{body: []byte("x")})
	require.Empty(t, disabled.entries)
	large := newImageCache(16<<20, time.Hour)
	large.put("x", response{body: make([]byte, (8<<20)+1)})
	require.Empty(t, large.entries)
}
