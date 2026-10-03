package imageutil

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPixelBudgetSmallImagesAndExclusiveLargeImage(t *testing.T) {
	b := NewBudget(6_000_000)
	a, releaseA := b.Scope(context.Background())
	c, releaseC := b.Scope(context.Background())
	require.NoError(t, reservePixels(a, 2_000_000))
	require.NoError(t, reservePixels(c, 3_000_000))
	deadline, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	large, releaseLarge := b.Scope(deadline)
	defer releaseLarge()
	require.ErrorIs(t, reservePixels(large, 20_000_000), context.DeadlineExceeded)
	releaseA()
	releaseC()
	large, releaseLarge = b.Scope(context.Background())
	require.NoError(t, reservePixels(large, 20_000_000))
	deadline, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	small, releaseSmall := b.Scope(deadline)
	defer releaseSmall()
	require.ErrorIs(t, reservePixels(small, 1), context.DeadlineExceeded)
	releaseLarge()
	releaseLarge() // cleanup is idempotent
	small, releaseSmall = b.Scope(context.Background())
	defer releaseSmall()
	require.NoError(t, reservePixels(small, 1))
}

func TestCancellationDoesNotReleaseAnActiveImage(t *testing.T) {
	b := NewBudget(100)
	ctx, cancel := context.WithCancel(context.Background())
	active, release := b.Scope(ctx)
	require.NoError(t, reservePixels(active, 100))
	cancel()
	deadline, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	waiting, done := b.Scope(deadline)
	defer done()
	require.ErrorIs(t, reservePixels(waiting, 1), context.DeadlineExceeded)
	release()
	next, releaseNext := b.Scope(context.Background())
	defer releaseNext()
	require.NoError(t, reservePixels(next, 100))
}

func TestDecodeWaitsBeforeAllocatingPixels(t *testing.T) {
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 10, 10))))
	b := NewBudget(100)
	first, release := b.Scope(context.Background())
	defer release()
	m, _, err := DecodeLimitedContext(first, bytes.NewReader(data.Bytes()))
	require.NoError(t, err)
	require.Equal(t, image.Rect(0, 0, 10, 10), m.Bounds())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	second, done := b.Scope(ctx)
	defer done()
	m, _, err = DecodeLimitedContext(second, bytes.NewReader(data.Bytes()))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, m)
	m, _, err = DecodeLimitedContext(ctx, bytes.NewReader(data.Bytes()))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, m)
}
