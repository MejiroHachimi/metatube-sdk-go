package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestGenreMatchingPreservesMetadata(t *testing.T) {
	tests := []struct {
		name     string
		excluded []string
		genres   []string
		want     []string
	}{
		{
			name: "default exclusions handle width and spacing",
			genres: []string{
				"１０８０Ｐ", "Ｂｌｕ－ｒａｙ（ブルーレイ）", "Blu\u00a0  ray",
				"Not 1080P", "４Ｋ", "Drama",
			},
			want: []string{"Not 1080P", "４Ｋ", "Drama"},
		},
		{
			name:     "custom exclusions use the same normalization",
			excluded: []string{" ４Ｋ ", "ＢＬＵ　　ＲＡＹ"},
			genres:   []string{"4k", "Blu ray", "4K restoration", "1080p"},
			want:     []string{"4K restoration", "1080p"},
		},
		{
			name:     "duplicates preserve first spelling and order",
			excluded: []string{""},
			genres: []string{
				"Ｄｒａｍａ", "drama", "Historic\u00a0  drama", "historic drama",
				"カメラ", "ｶﾒﾗ", "１０８０Ｐ", "1080p",
			},
			want: []string{"Ｄｒａｍａ", "Historic\u00a0  drama", "カメラ", "１０８０Ｐ"},
		},
		{
			name:   "fully excluded list stays an empty array",
			genres: []string{"１０８０Ｐ", "\u3000", ""},
			want:   []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := map[string]any{
				"genres": tt.genres, "title": "Ｄｒａｍａ", "summary": "4K restoration",
				"maker": "1080p", "label": "Blu-ray", "series": "４Ｋ",
				"actors":         []string{"Ａ", "A"},
				"preview_images": []string{"https://example.invalid/A", "https://example.invalid/a"},
			}
			raw, err := json.Marshal(map[string]any{"data": metadata})
			if err != nil {
				t.Fatal(err)
			}
			cfg := testConfig()
			if tt.excluded != nil {
				cfg.ExcludedGenres = tt.excluded
			}
			gateway := NewGateway(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, string(raw))
			}), cfg, nil, nil)
			response := request(gateway, "/v1/movies/FANZA/test")
			if response.Code != http.StatusOK {
				t.Fatalf("unexpected status %d", response.Code)
			}
			metadata["genres"] = tt.want
			want, err := json.Marshal(map[string]any{"data": metadata})
			if err != nil {
				t.Fatal(err)
			}
			var gotJSON, wantJSON any
			if err := json.Unmarshal(response.Body.Bytes(), &gotJSON); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(want, &wantJSON); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Fatal("genre filtering or metadata preservation differs from expected result")
			}
		})
	}
}
