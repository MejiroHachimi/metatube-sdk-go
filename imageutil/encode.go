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
