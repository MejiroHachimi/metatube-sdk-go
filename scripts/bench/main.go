package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"github.com/metatube-community/metatube-sdk-go/imageutil"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func source(w, h int) []byte {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	var seed uint32 = 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = 1664525*seed + 1013904223
			n := int(seed >> 27)
			m.SetRGBA(x, y, color.RGBA{uint8((x*191/w + n) % 256), uint8((y*191/h + n) % 256), uint8(((x+y)*150/(w+h) + n) % 256), 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, m, &jpeg.Options{Quality: 90}); err != nil {
		panic(err)
	}
	return b.Bytes()
}
func main() {
	w := flag.Int("w", 1200, "")
	h := flag.Int("h", 1800, "")
	c := flag.Int("c", 1, "")
	n := flag.Int("n", 8, "")
	budgetPixels := flag.Int64("budget", 0, "maximum decoded pixels in flight; 0 disables admission")
	flag.Parse()
	var budget *imageutil.Budget
	if *budgetPixels > 0 {
		budget = imageutil.NewBudget(*budgetPixels)
	}
	if err := imageutil.NativeWebP(); err != nil {
		panic(err)
	}
	data := source(*w, *h)
	runtime.GC()
	var peak atomic.Int64
	var maxBytes atomic.Int64
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(10 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				b, e := os.ReadFile("/sys/fs/cgroup/memory.current")
				if e == nil {
					v, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
					for old := peak.Load(); v > old; old = peak.Load() {
						if peak.CompareAndSwap(old, v) {
							break
						}
					}
				}
			}
		}
	}()
	durations := make(chan float64, *n)
	jobs := make(chan int, *n)
	for i := 0; i < *n; i++ {
		jobs <- i
	}
	close(jobs)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < *c; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				t := time.Now()
				ctx := context.Background()
				release := func() {}
				if budget != nil {
					ctx, release = budget.Scope(ctx)
				}
				m, _, e := imageutil.DecodeLimitedContext(ctx, bytes.NewReader(data))
				if e != nil {
					panic(e)
				}
				m = imageutil.CropImagePosition(m, 2.0/3, 1)
				var out bytes.Buffer
				if e = imageutil.EncodeToWebP(&out, m); e != nil {
					panic(e)
				}
				release()
				maxBytes.Store(int64(out.Len()))
				durations <- time.Since(t).Seconds()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()
	close(stop)
	close(durations)
	var ds []float64
	for d := range durations {
		ds = append(ds, d)
	}
	sort.Float64s(ds)
	status, _ := os.ReadFile("/proc/self/status")
	var hwm string
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			hwm = strings.TrimSpace(strings.TrimPrefix(line, "VmHWM:"))
		}
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"native": true, "pixel_budget": *budgetPixels, "width": *w, "height": *h, "concurrency": *c, "requests": *n, "elapsed_s": elapsed, "throughput_per_s": float64(*n) / elapsed, "median_request_s": ds[len(ds)/2], "slowest_request_s": ds[len(ds)-1], "peak_container_mib": float64(peak.Load()) / (1 << 20), "process_peak_rss": hwm, "source_bytes": len(data), "webp_bytes": maxBytes.Load(), "estimated_10000_minutes": elapsed / float64(*n) * 10000 / 60})
}
