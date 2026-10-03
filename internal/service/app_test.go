package service

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/lib/pq"
	"github.com/metatube-community/metatube-sdk-go/database"
	"github.com/metatube-community/metatube-sdk-go/model"
	"golang.org/x/image/webp"
	"gorm.io/gorm/logger"
)

func TestRealSDKSQLiteAndAuthentication(t *testing.T) {
	cfg := testConfig()
	cfg.DataDir = t.TempDir()
	cfg.Token = "test-only-token"
	handler, closeDB, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB()
	db, err := database.Open(&database.Config{DSN: filepath.Join(cfg.DataDir, "metadata.db"), LogLevel: logger.Silent})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	item := model.MovieInfo{ID: "test00001", Provider: "FANZA", Number: "TEST-001", Title: "<b>Example</b>", Homepage: "https://example.com/item", CoverURL: "https://example.com/cover.jpg", Genres: pq.StringArray{"1080P", "Blu-ray（ブルーレイ）", "Drama"}}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	path := "/v1/movies/FANZA/test00001?lazy=true"
	if w := request(handler, path); w.Code != 401 {
		t.Fatal("missing auth accepted", w.Code)
	}
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("Authorization", "Bearer test-only-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct{ Data model.MovieInfo }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Title != "Example" || len(result.Data.Genres) != 1 || result.Data.Genres[0] != "Drama" {
		t.Fatalf("filter not applied: %+v", result.Data)
	}
	var stored model.MovieInfo
	if err := db.Where("id = ? AND provider = ?", item.ID, item.Provider).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored.Genres) != 3 || stored.Title != item.Title {
		t.Fatal("response cleanup changed database")
	}
	if w := request(handler, "/readyz"); w.Code != 200 {
		t.Fatal("database not ready")
	}
	if w := request(handler, "/v1/providers"); w.Code != 200 {
		t.Fatal("providers failed")
	}
}
func TestConfigValidationAndEmptyExclusions(t *testing.T) {
	for _, k := range []string{"DATA_DIR", "DSN", "DATABASE_URL", "TOKEN", "PORT", "BIND", "IMAGE_CACHE_TTL", "REQUEST_TIMEOUT", "IMAGE_CACHE_MB", "MAX_CONCURRENT", "MAX_WAITING_REQUESTS", "METADATA_QUEUE_SIZE"} {
		t.Setenv(k, "")
	}
	t.Setenv("EXCLUDED_GENRES", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ExcludedGenres) != 1 || cfg.ExcludedGenres[0] != "" {
		t.Fatal("cannot disable exclusions")
	}
	t.Setenv("IMAGE_CACHE_MB", "-1")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("negative cache accepted")
	}
	t.Setenv("IMAGE_CACHE_MB", "0")
	t.Setenv("PORT", "65536")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("invalid port accepted")
	}
}

func TestRealImagePipeline(t *testing.T) {
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 30, 40)))
	}))
	defer upstream.Close()
	cfg := testConfig()
	cfg.DataDir = t.TempDir()
	handler, closeDB, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB()
	db, err := database.Open(&database.Config{DSN: filepath.Join(cfg.DataDir, "metadata.db"), LogLevel: logger.Silent})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	item := model.MovieInfo{ID: "test00002", Provider: "FANZA", Number: "TEST-002", Title: "Example", Homepage: "https://example.com/item", CoverURL: upstream.URL + "/cover.png"}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	path := "/v1/images/primary/FANZA/test00002?pos=1"
	for i, query := range []string{"", "&quality=1", "&quality=80", "&quality=100"} {
		w := request(handler, path+query)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if _, err := webp.Decode(bytes.NewReader(w.Body.Bytes())); err != nil {
			t.Fatal("not WebP", err)
		}
		if w.Header().Get("Content-Type") != "image/webp" {
			t.Fatal("wrong image content type", w.Header())
		}
		if i > 0 && w.Header().Get("X-Cache") != "HIT" {
			t.Fatal("no image cache hit")
		}
	}
	if hits.Load() != 1 {
		t.Fatal("image downloaded repeatedly", hits.Load())
	}
	if w := request(handler, path+"&url=http%3A%2F%2F127.0.0.1%2Fprivate"); w.Code != 400 {
		t.Fatal("arbitrary URL accepted", w.Code)
	}
}
