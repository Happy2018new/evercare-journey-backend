package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ImageFromBytes decodes PNG, JPEG, GIF, WebP, BMP, or TIFF image data.
func ImageFromBytes(data []byte) (image.Image, error) {
	decodedImage, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ImageFromBytes: %w", err)
	}
	return decodedImage, nil
}

// ImageToPNG encodes source as PNG data.
func ImageToPNG(source image.Image) ([]byte, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, source); err != nil {
		return nil, fmt.Errorf("ImageToPNG: %w", err)
	}
	return buffer.Bytes(), nil
}

// ResizeImage scales source to an image with exactly size*size pixels.
func ResizeImage(src image.Image, size int) (dst *image.RGBA) {
	dst = image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
