package hget

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadURL(t *testing.T) {
	testCases := []struct {
		name            string
		contentLength   int
		compress        string
		expectedError   bool
		expectedContent string
		httpStatus      int
	}{
		{
			name:            "no compression",
			contentLength:   1024,
			compress:        "",
			expectedError:   false,
			expectedContent: strings.Repeat("a", 1024),
		},
		{
			name:            "gzip compression",
			contentLength:   1024,
			compress:        "gzip",
			expectedError:   false,
			expectedContent: strings.Repeat("a", 1024),
		},
		{
			name:            "deflate compression",
			contentLength:   1024,
			compress:        "deflate",
			expectedError:   false,
			expectedContent: strings.Repeat("a", 1024),
		},
		{
			name:          "unsupported compression",
			contentLength: 1024,
			compress:      "br",
			expectedError: true,
		},
		{
			name:          "not found",
			contentLength: 1024,
			compress:      "",
			expectedError: true,
			httpStatus:    http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")

				if tc.httpStatus != http.StatusOK {
					w.WriteHeader(tc.httpStatus)
					return
				}

				var body io.Reader = bytes.NewBufferString(tc.expectedContent)
				if tc.compress == "gzip" {
					w.Header().Set("Content-Encoding", "gzip")
					var buf bytes.Buffer
					zw := gzip.NewWriter(&buf)
					_, _ = io.Copy(zw, body)
					_ = zw.Close()
					body = &buf
				} else if tc.compress == "deflate" {
					w.Header().Set("Content-Encoding", "deflate")
					var buf bytes.Buffer
					fw, _ := flate.NewWriter(&buf, flate.DefaultCompression)
					_, _ = io.Copy(fw, body)
					_ = fw.Close()
					body = &buf
				} else if tc.compress != "" {
					w.Header().Set("Content-Encoding", tc.compress)
				}

				w.WriteHeader(http.StatusOK)
				_, _ = io.Copy(w, body)
			}))
			defer server.Close()

			var output bytes.Buffer
			result, err := DownloadURL(context.Background(), server.URL, &output)

			if tc.expectedError {
				if err == nil {
					t.Fatalf("expected error, but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the hash
			hasher := sha256.New()
			hasher.Write([]byte(tc.expectedContent))
			expectedHash := hex.EncodeToString(hasher.Sum(nil))
			if result.Hash != expectedHash {
				t.Errorf("expected hash %s, but got %s", expectedHash, result.Hash)
			}

			// Verify the length
			if result.Length != int64(len(tc.expectedContent)) {
				t.Errorf("expected length %d, but got %d", len(tc.expectedContent), result.Length)
			}

			// Verify the content
			if output.String() != tc.expectedContent {
				t.Errorf("expected content %s, but got %s", tc.expectedContent, output.String())
			}
		})
	}
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}
