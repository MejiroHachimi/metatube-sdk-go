package imageutil

import (
	"image"
	"io"

	"github.com/gen2brain/webp"
)

// WebPQuality is fixed for all served images, independently of client parameters.
const WebPQuality = 80

func EncodeToWebP(w io.Writer, m image.Image) error {
	return webp.Encode(w, m, webp.Options{Quality: WebPQuality, Method: 4})
}

// NativeWebP reports whether the native libwebp backend is available.
func NativeWebP() error { return webp.Dynamic() }
