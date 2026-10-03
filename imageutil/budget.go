package imageutil

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/semaphore"
)

// Budget limits decoded images in flight, not compressed file sizes. A picture
// larger than the pixel budget runs exclusively. Weighted's FIFO waiters prevent
// a stream of small pictures from starving a large one.
type Budget struct {
	pixels int64
	sem    *semaphore.Weighted
}

func NewBudget(pixels int64) *Budget {
	if pixels <= 0 {
		panic("image pixel budget must be positive")
	}
	return &Budget{pixels: pixels, sem: semaphore.NewWeighted(pixels)}
}

type budgetKey struct{}
type reservation struct {
	budget *Budget
	held   int64
}

// Scope holds the reservation through decoding, cropping, watermarking and
// encoding. Only the processing worker may call release, after it stops using
// the image; a disconnected HTTP client must not release an active worker's RAM.
func (b *Budget) Scope(ctx context.Context) (context.Context, func()) {
	r := &reservation{budget: b}
	var once sync.Once
	return context.WithValue(ctx, budgetKey{}, r), func() {
		once.Do(func() {
			if r.held > 0 {
				b.sem.Release(r.held)
			}
		})
	}
}

func reservePixels(ctx context.Context, pixels int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r, ok := ctx.Value(budgetKey{}).(*reservation)
	if !ok {
		return nil
	}
	if r.held != 0 {
		return fmt.Errorf("image scope already holds a reservation")
	}
	weight := min(pixels, r.budget.pixels)
	if err := r.budget.sem.Acquire(ctx, weight); err != nil {
		return err
	}
	r.held = weight
	return nil
}
