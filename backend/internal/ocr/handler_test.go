package ocr_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"school-exam-studio/backend/internal/ocr"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.White)
	if err := png.Encode(&body, img); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestValidateImage(t *testing.T) {
	if contentType, err := ocr.ValidateImage(testPNG(t)); err != nil || contentType != "image/png" {
		t.Fatalf("valid PNG contentType=%q err=%v", contentType, err)
	}
	var jpegBody bytes.Buffer
	if err := jpeg.Encode(&jpegBody, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil); err != nil {
		t.Fatal(err)
	}
	if contentType, err := ocr.ValidateImage(jpegBody.Bytes()); err != nil || contentType != "image/jpeg" {
		t.Fatalf("valid JPEG contentType=%q err=%v", contentType, err)
	}
	webp, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	if contentType, err := ocr.ValidateImage(webp); err != nil || contentType != "image/webp" {
		t.Fatalf("valid WebP contentType=%q err=%v", contentType, err)
	}
	if _, err := ocr.ValidateImage([]byte("GIF89a-not-a-real-image")); err == nil {
		t.Fatal("non-image data should be rejected")
	}
	if _, err := ocr.ValidateImage(make([]byte, ocr.MaxImageBytes+1)); err == nil {
		t.Fatal("oversized image should be rejected")
	}
	tooManyPixels := pngIHDR(5001, 4000)
	if _, err := ocr.ValidateImage(tooManyPixels); err != ocr.ErrTooManyPixels {
		t.Fatalf("expected pixel limit error, got %v", err)
	}
}

func pngIHDR(width, height uint32) []byte {
	var body bytes.Buffer
	body.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], width)
	binary.BigEndian.PutUint32(data[4:8], height)
	data[8] = 8
	data[9] = 2
	writePNGChunk(&body, "IHDR", data)
	writePNGChunk(&body, "IEND", nil)
	return body.Bytes()
}

func writePNGChunk(body *bytes.Buffer, kind string, data []byte) {
	_ = binary.Write(body, binary.BigEndian, uint32(len(data)))
	body.WriteString(kind)
	body.Write(data)
	checksum := crc32.NewIEEE()
	_, _ = checksum.Write([]byte(kind))
	_, _ = checksum.Write(data)
	_ = binary.Write(body, binary.BigEndian, checksum.Sum32())
}
