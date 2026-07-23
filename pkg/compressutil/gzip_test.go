package compressutil

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"
)

func TestGzipRoundTripsDeterministicallyWithoutMutatingInput(t *testing.T) {
	original := []byte(`{"status":"ok","result":{"value":"天气"}}`)
	input := append([]byte(nil), original...)
	first, err := Gzip(input)
	if err != nil {
		t.Fatalf("Gzip() error=%v", err)
	}
	second, err := Gzip(input)
	if err != nil {
		t.Fatalf("Gzip() second error=%v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("Gzip() output is not deterministic")
	}
	if !bytes.Equal(input, original) {
		t.Fatalf("Gzip() mutated input=%q", input)
	}
	reader, err := gzip.NewReader(bytes.NewReader(first))
	if err != nil {
		t.Fatalf("gzip.NewReader() error=%v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip payload: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close gzip reader: %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatalf("decoded=%q want=%q", decoded, original)
	}
}

func TestGzipAcceptsEmptyPayload(t *testing.T) {
	compressed, err := Gzip(nil)
	if err != nil || len(compressed) == 0 {
		t.Fatalf("Gzip(nil) length=%d error=%v", len(compressed), err)
	}
}

func TestGunzipDecodesWithoutMutatingInput(t *testing.T) {
	original := []byte(`{"status":"ok","result":{"value":"天气"}}`)
	compressed, err := Gzip(original)
	if err != nil {
		t.Fatalf("Gzip() error=%v", err)
	}
	input := append([]byte(nil), compressed...)

	decoded, err := Gunzip(input)
	if err != nil {
		t.Fatalf("Gunzip() error=%v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatalf("Gunzip()=%q want %q", decoded, original)
	}
	if !bytes.Equal(input, compressed) {
		t.Fatal("Gunzip() mutated input")
	}
}

func TestGunzipRejectsInvalidPayload(t *testing.T) {
	if _, err := Gunzip([]byte("not-gzip")); err == nil {
		t.Fatal("Gunzip() accepted invalid gzip payload")
	}
}
