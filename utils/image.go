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

// ImageConfigFromBytes reads an image's dimensions without decoding its pixels.
func ImageConfigFromBytes(data []byte) (image.Config, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return image.Config{}, err
	}
	return config, nil
}

// ImageToPNG encodes source as PNG data.
func ImageToPNG(source image.Image) ([]byte, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, source); err != nil {
		return nil, fmt.Errorf("ImageToPNG: %w", err)
	}
	return buffer.Bytes(), nil
}

// ResizeImageTo256 scales source to an image with exactly 256 by 256 pixels.
func ResizeImageTo256(src image.Image) (dst *image.RGBA) {
	dst = image.NewRGBA(image.Rect(0, 0, 256, 256))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
