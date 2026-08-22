package utils

import (
	"bytes"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
)

// CompressBrotli compresses data using Brotli.
func CompressBrotli(data []byte) ([]byte, error) {
	result := bytes.NewBuffer(nil)
	writer := brotli.NewWriter(result)
	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("CompressBrotli: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("CompressBrotli: %w", err)
	}
	return result.Bytes(), nil
}

// DecompressBrotli decompresses data and reports whether it exceeds maxSize bytes.
func DecompressBrotli(src []byte, maxSize int64) (dst []byte, exceeded bool, err error) {
	reader := io.LimitReader(brotli.NewReader(bytes.NewReader(src)), maxSize+1)
	dst, err = io.ReadAll(reader)
	if err != nil {
		return nil, false, fmt.Errorf("DecompressBrotli: %w", err)
	}
	return dst, len(dst) > int(maxSize), nil
}
