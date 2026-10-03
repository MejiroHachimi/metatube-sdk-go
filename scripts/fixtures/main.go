// This executable is included only in the loadtest Docker target.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/metatube-community/metatube-sdk-go/database"
	"github.com/metatube-community/metatube-sdk-go/internal/service"
	"github.com/metatube-community/metatube-sdk-go/model"
	"gorm.io/gorm/logger"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "seed" {
		seed()
		return
	}
	log.Fatal(http.ListenAndServe("127.0.0.1:18081", http.FileServer(http.Dir("/tmp/fixtures"))))
}

func seed() {
	cfg, err := service.LoadConfig()
	must(err)
	_, closeDB, err := service.Open(cfg)
	must(err)
	must(closeDB())
	if cfg.DSN == "" {
		cfg.DSN = filepath.Join(cfg.DataDir, "metadata.db")
	}
	db, err := database.Open(&database.Config{DSN: cfg.DSN, LogLevel: logger.Silent})
	must(err)
	sqlDB, err := db.DB()
	must(err)
	defer sqlDB.Close()
	must(os.MkdirAll("/tmp/fixtures", 0755))
	for _, spec := range []struct {
		name string
		w, h int
		png  bool
	}{
		{"small", 1200, 1800, false}, {"medium", 2000, 3000, false},
		{"large", 4000, 5000, false}, {"png", 4000, 5000, true},
	} {
		filename := spec.name + ".jpg"
		if spec.png {
			filename = spec.name + ".png"
		}
		writeFixture(filepath.Join("/tmp/fixtures", filename), spec.w, spec.h, spec.png)
		debug.FreeOSMemory()
		for i := 0; i < 40; i++ {
			item := model.MovieInfo{ID: fmt.Sprintf("%s%05d", spec.name, i), Provider: "FANZA", Number: fmt.Sprintf("TEST-%d", i), Title: "Load test fixture", Homepage: "https://example.com/fixture", CoverURL: "http://127.0.0.1:18081/" + filename}
			must(db.Save(&item).Error)
		}
	}
	log.Print("Test fixtures prepared")
}

func writeFixture(path string, w, h int, usePNG bool) {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	var seed uint32 = 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = 1664525*seed + 1013904223
			n := int(seed >> 27)
			if usePNG {
				n = 0
			} // A small compressed file with a large decoded resolution.
			m.SetRGBA(x, y, color.RGBA{uint8((x*191/w + n) % 256), uint8((y*191/h + n) % 256), uint8(((x+y)*150/(w+h) + n) % 256), 255})
		}
	}
	f, err := os.Create(path)
	must(err)
	if usePNG {
		must(png.Encode(f, m))
	} else {
		must(jpeg.Encode(f, m, &jpeg.Options{Quality: 90}))
	}
	must(f.Close())
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
