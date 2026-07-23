package compressutil

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
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

// Gunzip returns the decoded gzip payload without mutating data.
func Gunzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("compressutil: create gzip reader: %w", err)
	}
	defer reader.Close()

	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("compressutil: read gzip payload: %w", err)
	}
	return payload, nil
}
