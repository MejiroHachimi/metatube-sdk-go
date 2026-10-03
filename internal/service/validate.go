package service

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func validate(r *http.Request) (bool, error) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return false, fmt.Errorf("invalid query encoding")
	}
	if len(r.URL.RawQuery) > 8192 {
		return false, fmt.Errorf("query exceeds 8192 bytes")
	}
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	image := len(path) == 5 && path[0] == "v1" && path[1] == "images"
	allowed := map[string]bool{}
	switch {
	case r.URL.Path == "/" || r.URL.Path == "/v1/providers" || r.URL.Path == "/v1/db/version":
	case image:
		if path[2] != "primary" && path[2] != "thumb" && path[2] != "backdrop" {
			return image, fmt.Errorf("unknown image kind")
		}
		for _, k := range []string{"url", "ratio", "pos", "auto", "badge", "quality"} {
			allowed[k] = true
		}
	case len(path) == 3 && path[0] == "v1" && (path[1] == "movies" || path[1] == "actors") && path[2] == "search":
		for _, k := range []string{"q", "provider", "fallback"} {
			allowed[k] = true
		}
		if strings.TrimSpace(q.Get("q")) == "" {
			return false, fmt.Errorf("q is required")
		}
	case len(path) == 4 && path[0] == "v1" && (path[1] == "movies" || path[1] == "actors"):
		allowed["lazy"] = true
	case len(path) == 4 && path[0] == "v1" && path[1] == "reviews":
		allowed["lazy"] = true
		allowed["homepage"] = true
	default:
		return false, errNotFound
	}
	for k, v := range q {
		if !allowed[k] {
			return image, fmt.Errorf("unknown parameter: %s", k)
		}
		if len(v) != 1 {
			return image, fmt.Errorf("parameter must appear once: %s", k)
		}
	}
	for _, k := range []string{"lazy", "fallback", "auto"} {
		if v := q.Get(k); v != "" {
			b, e := strconv.ParseBool(v)
			if e != nil {
				return image, fmt.Errorf("%s must be a boolean", k)
			}
			q.Set(k, strconv.FormatBool(b))
		}
	}
	if v := q.Get("quality"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100 {
			return image, fmt.Errorf("quality must be between 1 and 100")
		}
		// Accept legacy quality parameters, but always encode at quality 80.
		q.Del("quality")
	}
	for _, k := range []string{"ratio", "pos"} {
		if v := q.Get(k); v != "" {
			n, e := strconv.ParseFloat(v, 64)
			valid := n == -1 || (k == "ratio" && n >= 0.1 && n <= 10) || (k == "pos" && n >= 0 && n <= 1)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || !valid {
				return image, fmt.Errorf("invalid %s", k)
			}
			q.Set(k, strconv.FormatFloat(n, 'g', -1, 64))
		}
	}
	if v := q.Get("badge"); v != "" && v != "zimu.png" && v != "u.png" && v != "uc.png" {
		return image, fmt.Errorf("badge must be zimu.png, u.png or uc.png")
	}
	for k, v := range q {
		if v[0] == "" {
			q.Del(k)
		}
	}
	r.URL.RawQuery = q.Encode()
	return image, nil
}
