package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

var errNotFound = errors.New("not found")

const maxResponseBytes = 16 << 20

type response struct {
	status int
	header http.Header
	body   []byte
}
type capture struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func (c *capture) Header() http.Header { return c.header }
func (c *capture) WriteHeader(s int) {
	if c.status == 0 {
		c.status = s
	}
}
func (c *capture) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = 200
	}
	if c.body.Len()+len(b) > maxResponseBytes {
		c.overflow = true
		return 0, fmt.Errorf("response too large")
	}
	return c.body.Write(b)
}
func errorResponse(code int, message string) response {
	b, _ := json.Marshal(map[string]any{"error": map[string]any{"code": code, "message": message}})
	return response{code, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}, "Cache-Control": []string{"no-store"}}, b}
}

type Gateway struct {
	next          http.Handler
	cache         *imageCache
	flight        singleflight.Group
	slots         chan struct{}
	config        Config
	excluded      map[string]bool
	validateImage func(*http.Request) error
	ready         func(context.Context) error
}

func NewGateway(next http.Handler, c Config, checkImage func(*http.Request) error, ready func(context.Context) error) *Gateway {
	excluded := map[string]bool{}
	for _, s := range c.ExcludedGenres {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			excluded[s] = true
		}
	}
	return &Gateway{next: next, cache: newImageCache(c.CacheBytes, c.CacheTTL), slots: make(chan struct{}, c.MaxConcurrent), config: c, excluded: excluded, validateImage: checkImage, ready: ready}
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		writeResponse(w, r, errorResponse(405, "method not allowed"))
		return
	}
	switch r.URL.Path {
	case "/healthz":
		writeResponse(w, r, jsonResponse(map[string]any{"status": "ok"}))
		return
	case "/readyz":
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if g.ready != nil && g.ready(ctx) != nil {
			writeResponse(w, r, errorResponse(503, "database not ready"))
		} else {
			writeResponse(w, r, jsonResponse(map[string]any{"status": "ready"}))
		}
		return
	case "/openapi.json":
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "HEAD" {
			_, _ = w.Write(openapiJSON)
		}
		return
	case "/docs", "/docs/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		if r.Method != "HEAD" {
			_, _ = w.Write(docsHTML)
		}
		return
	}
	// Clone URL before normalizing parameters; callers may reuse their request.
	r = r.Clone(r.Context())
	u := *r.URL
	r.URL = &u
	image, err := validate(r)
	if err != nil {
		code := 400
		message := err.Error()
		if errors.Is(err, errNotFound) {
			code = 404
			message = "endpoint not found"
		}
		writeResponse(w, r, errorResponse(code, message))
		return
	}
	if !image && r.Method == "HEAD" {
		w.Header().Set("Allow", "GET")
		writeResponse(w, r, errorResponse(405, "method not allowed"))
		return
	}
	key := r.URL.Path + "?" + r.URL.RawQuery
	if image {
		if v, ok := g.cache.get(key); ok {
			v.header = v.header.Clone()
			v.header.Set("X-Cache", "HIT")
			writeResponse(w, r, v)
			return
		}
	}
	request := r.Clone(context.WithoutCancel(r.Context()))
	request.Method = "GET"
	var result <-chan singleflight.Result
	if image {
		result = g.flight.DoChan(key, func() (any, error) {
			if v, ok := g.cache.get(key); ok {
				return v, nil
			}
			v := g.compute(request, image)
			if v.status == 200 {
				g.cache.put(key, v)
			}
			return v, nil
		})
	} else {
		ch := make(chan singleflight.Result, 1)
		result = ch
		go func() { ch <- singleflight.Result{Val: g.compute(request, false)} }()
	}
	timer := time.NewTimer(g.config.RequestTimeout)
	defer timer.Stop()
	select {
	case got := <-result:
		v := got.Val.(response)
		if image && v.status == 200 {
			v.header = v.header.Clone()
			v.header.Set("X-Cache", "MISS")
		}
		writeResponse(w, r, v)
	case <-timer.C:
		writeResponse(w, r, errorResponse(504, "upstream request timed out"))
	case <-r.Context().Done():
		return
	}
}
func (g *Gateway) compute(r *http.Request, image bool) (out response) {
	select {
	case g.slots <- struct{}{}:
		defer func() { <-g.slots }()
	default:
		return errorResponse(503, "server busy; retry later")
	}
	defer func() {
		if recover() != nil {
			out = errorResponse(500, "internal server error")
		}
	}()
	if image && g.validateImage != nil {
		if err := g.validateImage(r); err != nil {
			return errorResponse(400, "image URL must belong to the requested item")
		}
	}
	c := &capture{header: make(http.Header)}
	g.next.ServeHTTP(c, r)
	if c.overflow {
		return errorResponse(502, "upstream response exceeds size limit")
	}
	if c.status == 0 {
		c.status = 200
	}
	if c.status >= 400 {
		message := http.StatusText(c.status)
		if c.status >= 500 {
			message = "upstream request failed"
		}
		return errorResponse(c.status, message)
	}
	if c.status != 200 {
		return errorResponse(502, "unexpected upstream response")
	}
	header := make(http.Header)
	header.Set("Cache-Control", "no-store")
	body := c.body.Bytes()
	if image {
		if !strings.HasPrefix(c.header.Get("Content-Type"), "image/jpeg") {
			return errorResponse(502, "invalid upstream image")
		}
		header.Set("Content-Type", "image/jpeg")
		header.Set("Cache-Control", fmt.Sprintf("public, max-age=3600, s-maxage=%d", int(g.config.CacheTTL.Seconds())))
		for _, k := range []string{"X-MetaTube-Image-Width", "X-MetaTube-Image-Height"} {
			if v := c.header.Get(k); v != "" {
				header.Set(k, v)
			}
		}
		sum := sha256.Sum256(body)
		header.Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
	} else {
		var err error
		body, err = cleanJSON(body, g.excluded)
		if err != nil {
			return errorResponse(502, "invalid upstream JSON")
		}
		header.Set("Content-Type", "application/json; charset=utf-8")
	}
	return response{200, header, body}
}
func jsonResponse(data any) response {
	b, _ := json.Marshal(map[string]any{"data": data})
	return response{200, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}, "Cache-Control": []string{"no-store"}}, b}
}
func writeResponse(w http.ResponseWriter, r *http.Request, v response) {
	for k, values := range v.header {
		w.Header()[k] = append([]string(nil), values...)
	}
	if etag := v.header.Get("ETag"); etag != "" && v.status == 200 {
		for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
				w.WriteHeader(304)
				return
			}
		}
	}
	w.Header().Set("Content-Length", fmt.Sprint(len(v.body)))
	w.WriteHeader(v.status)
	if r.Method != "HEAD" {
		_, _ = w.Write(v.body)
	}
}
