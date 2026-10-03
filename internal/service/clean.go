package service

import (
	"encoding/json"
	"html"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/text/unicode/norm"
)

var plainText = bluemonday.StrictPolicy()
var listFields = map[string]bool{"genres": true, "actors": true, "aliases": true, "images": true, "preview_images": true}
var textFields = map[string]bool{"title": true, "summary": true, "name": true, "comment": true, "author": true, "hobby": true, "skill": true, "director": true, "maker": true, "label": true, "series": true}

// genreKey normalizes comparison only; surviving values keep their spelling.
func genreKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(norm.NFKC.String(s)), " "))
}

func cleanJSON(body []byte, excluded map[string]bool) ([]byte, error) {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, err
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if listFields[k] {
					out := []string{}
					seen := map[string]bool{}
					if items, ok := v.([]any); ok {
						for _, item := range items {
							s, ok := item.(string)
							if !ok {
								continue
							}
							if k != "images" && k != "preview_images" {
								s = html.UnescapeString(plainText.Sanitize(s))
							}
							s = strings.TrimSpace(s)
							key := strings.ToLower(s)
							if k == "genres" {
								key = genreKey(s)
							} else if k == "images" || k == "preview_images" {
								key = s
							}
							if s == "" || seen[key] || (k == "genres" && excluded[key]) {
								continue
							}
							seen[key] = true
							out = append(out, s)
						}
					}
					x[k] = out
				} else if s, ok := v.(string); ok && textFields[k] {
					x[k] = strings.TrimSpace(html.UnescapeString(plainText.Sanitize(s)))
				} else {
					walk(v)
				}
			}
		case []any:
			for _, v := range x {
				walk(v)
			}
		}
	}
	walk(value)
	return json.Marshal(value)
}
