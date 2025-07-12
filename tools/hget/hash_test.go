package hget

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestGetHash(t *testing.T) {
	testCases := []struct {
		name         string
		input        string
		expectedHash string
		expectError  bool
	}{
		{
			name:         "empty string",
			input:        "",
			expectedHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:         "simple string",
			input:        "hello world",
			expectedHash: "b7f783a6187e73d3d4949618404a9a0d258a2a5502782584944247b61f7c6701",
		},
		{
			name:         "long string",
			input:        strings.Repeat("a", 1024),
			expectedHash: "9771447089182511111817977777777777777777777777777777777777777777",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := bytes.NewBufferString(tc.input)
			hash, err := GetHash(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hash != tc.expectedHash {
				t.Errorf("expected hash %s, but got %s", tc.expectedHash, hash)
			}
		})
	}
}

func TestGetHashForFile(t *testing.T) {
	testCases := []struct {
		name         string
		content      string
		expectedHash string
		expectError  bool
	}{
		{
			name:         "empty file",
			content:      "",
			expectedHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:         "simple file",
			content:      "hello world",
			expectedHash: "b7f783a6187e73d3d4949618404a9a0d258a2a5502782584944247b61f7c6701",
		},
		{
			name:         "long file",
			content:      strings.Repeat("a", 1024),
			expectedHash: "9771447089182511111817977777777777777777777777777777777777777777",
		},
		{
			name:        "non-existent file",
			content:     "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tmpFile *os.File
			if tc.name != "non-existent file" {
				tmpFile, err := os.CreateTemp("", "testfile")
				if err != nil {
					t.Fatalf("failed to create temp file: %v", err)
				}
				defer os.Remove(tmpFile.Name())
				defer tmpFile.Close()

				_, err = tmpFile.WriteString(tc.content)
				if err != nil {
					t.Fatalf("failed to write to temp file: %v", err)
				}
			}

			var hash string
			var err error
			if tc.name == "non-existent file" {
				hash, err = GetHashForFile("non-existent-file")
			} else {
				hash, err = GetHashForFile(tmpFile.Name())
			}

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if hash != tc.expectedHash {
				t.Errorf("expected hash %s, but got %s", tc.expectedHash, hash)
			}
		})
	}
}

func calculateSHA256(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}
