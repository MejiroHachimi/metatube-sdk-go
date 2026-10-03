package imageutil

import (
	"bytes"
	"fmt"
	"image"
	"io"
)

// DecodeLimited bounds both compressed bytes and decoded pixel count before allocation.
func DecodeLimited(r io.Reader) (image.Image, string, error) {
	const maxBytes = 16 << 20
	const maxPixels = 20_000_000
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxBytes {
		return nil, "", fmt.Errorf("image exceeds 16 MiB")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, "", fmt.Errorf("image exceeds pixel limit")
	}
	return Decode(bytes.NewReader(data))
}
