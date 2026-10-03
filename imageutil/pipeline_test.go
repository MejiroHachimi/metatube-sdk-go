package imageutil

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"testing"

	encoder "github.com/gen2brain/webp"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/webp"
)

func testImage() *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			m.SetNRGBA(x, y, color.NRGBA{uint8(x * 4), uint8(y * 5), uint8((x * y) % 256), 255})
		}
	}
	return m
}

func TestWebPEncodingQualityAndRoundTrip(t *testing.T) {
	m := testImage()
	var encoded, expected bytes.Buffer
	require.NoError(t, EncodeToWebP(&encoded, m))
	require.Equal(t, "RIFF", string(encoded.Bytes()[:4]))
	require.Equal(t, "WEBP", string(encoded.Bytes()[8:12]))
	decoded, err := webp.Decode(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	require.Equal(t, m.Bounds(), decoded.Bounds())
	// Pin the requested quality independently of the production constant.
	require.NoError(t, encoder.Encode(&expected, m, encoder.Options{Quality: 80, Method: 4}))
	require.Equal(t, expected.Bytes(), encoded.Bytes())
	require.Error(t, EncodeToWebP(brokenWriter{}, m))
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestLimitedDecodeFormatsAndErrors(t *testing.T) {
	for _, format := range []string{"png", "jpeg", "webp"} {
		t.Run(format, func(t *testing.T) {
			var b bytes.Buffer
			switch format {
			case "png":
				require.NoError(t, png.Encode(&b, testImage()))
			case "jpeg":
				require.NoError(t, jpeg.Encode(&b, testImage(), nil))
			case "webp":
				require.NoError(t, EncodeToWebP(&b, testImage()))
			}
			m, actual, err := DecodeLimited(&b)
			require.NoError(t, err)
			require.Equal(t, format, actual)
			require.Equal(t, testImage().Bounds(), m.Bounds())
		})
	}
	_, _, err := DecodeLimited(brokenReader{})
	require.True(t, errors.Is(err, io.ErrUnexpectedEOF))
	_, _, err = DecodeLimited(bytes.NewBufferString("not an image"))
	require.Error(t, err)
}

func TestCropPositionsAndResize(t *testing.T) {
	m := image.NewNRGBA(image.Rect(10, 20, 110, 70))
	for _, tc := range []struct {
		ratio, pos float64
		want       image.Rectangle
	}{
		{1, 0, image.Rect(10, 20, 60, 70)},
		{1, 1, image.Rect(60, 20, 110, 70)},
		{4, 0, image.Rect(10, 20, 110, 45)},
		{4, 1, image.Rect(10, 45, 110, 70)},
		{-1, 0, m.Bounds()}, {101, 0, m.Bounds()},
	} {
		require.Equal(t, tc.want, CropImagePosition(m, tc.ratio, tc.pos).Bounds())
	}
	require.Same(t, m, Resize(m, 0, 0))
	for _, size := range [][2]int{{20, 0}, {0, 10}, {20, 10}} {
		require.Equal(t, image.Rect(0, 0, 20, 10), Resize(m, size[0], size[1]).Bounds())
	}
}

func TestWatermarkAndImageHashes(t *testing.T) {
	m := testImage()
	before := m.NRGBAAt(0, 0)
	mark := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	mark.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	out := Watermark(m, mark, image.Point{})
	require.Equal(t, color.NRGBA{R: 255, A: 255}, color.NRGBAModel.Convert(out.At(0, 0)))
	require.Equal(t, before, m.NRGBAAt(0, 0), "watermark must not mutate the source")
	require.Equal(t, 0, AverageHashDistance(m, m))
	require.Equal(t, 0, DifferenceHashDistance(m, m))
	require.Equal(t, 0, PerceptionHashDistance(m, m))
	require.True(t, Similar(m, m))
}

func TestNativeBackendWhenRequired(t *testing.T) {
	if os.Getenv("REQUIRE_NATIVE_WEBP") == "1" {
		require.NoError(t, NativeWebP())
	}
}
