package compressutil

import (
	"bytes"
	"compress/gzip"
	"fmt"
)

// Gzip returns a deterministic gzip encoding of data without mutating data.
func Gzip(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("compressutil: write gzip payload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("compressutil: close gzip writer: %w", err)
	}
	return buffer.Bytes(), nil
}
