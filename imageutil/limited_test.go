package imageutil

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestDecodeLimited(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if img, _, err := DecodeLimited(bytes.NewReader(b.Bytes())); err != nil || img.Bounds().Dx() != 2 {
		t.Fatal("valid image rejected", err)
	}
	data := append([]byte(nil), b.Bytes()...)
	binary.BigEndian.PutUint32(data[16:20], 100000)
	binary.BigEndian.PutUint32(data[20:24], 100000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	if _, _, err := DecodeLimited(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "pixel limit") {
		t.Fatal("oversized dimensions accepted", err)
	}
	if _, _, err := DecodeLimited(strings.NewReader(strings.Repeat("x", (16<<20)+1))); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatal("oversized body accepted", err)
	}
}
