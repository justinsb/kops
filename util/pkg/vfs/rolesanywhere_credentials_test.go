/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vfs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCertificate creates a self-signed certificate and private key for testing
func generateTestCertificate(t *testing.T) (certPEM, keyPEM []byte) {
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Organization"},
			CommonName:   "test-certificate",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Encode certificate to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	// Encode private key to PEM
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("Failed to marshal private key: %v", err)
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	return certPEM, keyPEM
}

func TestGetRolesAnywhereConfig(t *testing.T) {
	tests := []struct {
		name      string
		envVars   map[string]string
		expectNil bool
	}{
		{
			name: "all required env vars set",
			envVars: map[string]string{
				"AWS_ROLES_ANYWHERE_CERT_PATH":        "/path/to/cert.pem",
				"AWS_ROLES_ANYWHERE_KEY_PATH":         "/path/to/key.pem",
				"AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN": "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/abc123",
				"AWS_ROLES_ANYWHERE_PROFILE_ARN":      "arn:aws:rolesanywhere:us-east-1:123456789012:profile/def456",
				"AWS_ROLES_ANYWHERE_ROLE_ARN":         "arn:aws:iam::123456789012:role/TestRole",
				"AWS_ROLES_ANYWHERE_REGION":           "us-west-2",
			},
			expectNil: false,
		},
		{
			name: "missing cert path",
			envVars: map[string]string{
				"AWS_ROLES_ANYWHERE_KEY_PATH":         "/path/to/key.pem",
				"AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN": "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/abc123",
				"AWS_ROLES_ANYWHERE_PROFILE_ARN":      "arn:aws:rolesanywhere:us-east-1:123456789012:profile/def456",
				"AWS_ROLES_ANYWHERE_ROLE_ARN":         "arn:aws:iam::123456789012:role/TestRole",
			},
			expectNil: true,
		},
		{
			name: "region defaults to us-east-1",
			envVars: map[string]string{
				"AWS_ROLES_ANYWHERE_CERT_PATH":        "/path/to/cert.pem",
				"AWS_ROLES_ANYWHERE_KEY_PATH":         "/path/to/key.pem",
				"AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN": "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/abc123",
				"AWS_ROLES_ANYWHERE_PROFILE_ARN":      "arn:aws:rolesanywhere:us-east-1:123456789012:profile/def456",
				"AWS_ROLES_ANYWHERE_ROLE_ARN":         "arn:aws:iam::123456789012:role/TestRole",
			},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			os.Unsetenv("AWS_ROLES_ANYWHERE_CERT_PATH")
			os.Unsetenv("AWS_ROLES_ANYWHERE_KEY_PATH")
			os.Unsetenv("AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN")
			os.Unsetenv("AWS_ROLES_ANYWHERE_PROFILE_ARN")
			os.Unsetenv("AWS_ROLES_ANYWHERE_ROLE_ARN")
			os.Unsetenv("AWS_ROLES_ANYWHERE_REGION")

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			config := getRolesAnywhereConfig()

			if tt.expectNil {
				if config != nil {
					t.Errorf("Expected nil config, got %+v", config)
				}
			} else {
				if config == nil {
					t.Error("Expected non-nil config, got nil")
				} else {
					// Verify config values
					if config.certPath != tt.envVars["AWS_ROLES_ANYWHERE_CERT_PATH"] {
						t.Errorf("certPath = %v, want %v", config.certPath, tt.envVars["AWS_ROLES_ANYWHERE_CERT_PATH"])
					}
					if config.keyPath != tt.envVars["AWS_ROLES_ANYWHERE_KEY_PATH"] {
						t.Errorf("keyPath = %v, want %v", config.keyPath, tt.envVars["AWS_ROLES_ANYWHERE_KEY_PATH"])
					}
					if config.trustAnchorARN != tt.envVars["AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN"] {
						t.Errorf("trustAnchorARN = %v, want %v", config.trustAnchorARN, tt.envVars["AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN"])
					}
					if config.profileARN != tt.envVars["AWS_ROLES_ANYWHERE_PROFILE_ARN"] {
						t.Errorf("profileARN = %v, want %v", config.profileARN, tt.envVars["AWS_ROLES_ANYWHERE_PROFILE_ARN"])
					}
					if config.roleARN != tt.envVars["AWS_ROLES_ANYWHERE_ROLE_ARN"] {
						t.Errorf("roleARN = %v, want %v", config.roleARN, tt.envVars["AWS_ROLES_ANYWHERE_ROLE_ARN"])
					}

					// Check region default
					expectedRegion := tt.envVars["AWS_ROLES_ANYWHERE_REGION"]
					if expectedRegion == "" {
						expectedRegion = "us-east-1"
					}
					if config.region != expectedRegion {
						t.Errorf("region = %v, want %v", config.region, expectedRegion)
					}
				}
			}
		})
	}
}

func TestIsRolesAnywhereConfigured(t *testing.T) {
	// Clear env vars
	os.Unsetenv("AWS_ROLES_ANYWHERE_CERT_PATH")
	os.Unsetenv("AWS_ROLES_ANYWHERE_KEY_PATH")
	os.Unsetenv("AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN")
	os.Unsetenv("AWS_ROLES_ANYWHERE_PROFILE_ARN")
	os.Unsetenv("AWS_ROLES_ANYWHERE_ROLE_ARN")

	if isRolesAnywhereConfigured() {
		t.Error("Expected isRolesAnywhereConfigured() to return false when env vars not set")
	}

	// Set env vars
	os.Setenv("AWS_ROLES_ANYWHERE_CERT_PATH", "/path/to/cert.pem")
	os.Setenv("AWS_ROLES_ANYWHERE_KEY_PATH", "/path/to/key.pem")
	os.Setenv("AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN", "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/abc123")
	os.Setenv("AWS_ROLES_ANYWHERE_PROFILE_ARN", "arn:aws:rolesanywhere:us-east-1:123456789012:profile/def456")
	os.Setenv("AWS_ROLES_ANYWHERE_ROLE_ARN", "arn:aws:iam::123456789012:role/TestRole")

	if !isRolesAnywhereConfigured() {
		t.Error("Expected isRolesAnywhereConfigured() to return true when all env vars are set")
	}

	// Cleanup
	os.Unsetenv("AWS_ROLES_ANYWHERE_CERT_PATH")
	os.Unsetenv("AWS_ROLES_ANYWHERE_KEY_PATH")
	os.Unsetenv("AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN")
	os.Unsetenv("AWS_ROLES_ANYWHERE_PROFILE_ARN")
	os.Unsetenv("AWS_ROLES_ANYWHERE_ROLE_ARN")
}

func TestNewRolesAnywhereCredentialsProvider(t *testing.T) {
	// Create temporary directory for test certificates
	tempDir, err := os.MkdirTemp("", "rolesanywhere-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate test certificate and key
	certPEM, keyPEM := generateTestCertificate(t)

	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")

	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatalf("Failed to write certificate: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatalf("Failed to write private key: %v", err)
	}

	tests := []struct {
		name        string
		certPath    string
		keyPath     string
		expectError bool
	}{
		{
			name:        "valid certificate and key",
			certPath:    certPath,
			keyPath:     keyPath,
			expectError: false,
		},
		{
			name:        "non-existent certificate",
			certPath:    "/nonexistent/cert.pem",
			keyPath:     keyPath,
			expectError: true,
		},
		{
			name:        "non-existent key",
			certPath:    certPath,
			keyPath:     "/nonexistent/key.pem",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewRolesAnywhereCredentialsProvider(
				tt.certPath,
				tt.keyPath,
				"arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/abc123",
				"arn:aws:rolesanywhere:us-east-1:123456789012:profile/def456",
				"arn:aws:iam::123456789012:role/TestRole",
				"us-east-1",
			)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if provider == nil {
					t.Error("Expected non-nil provider")
				}
			}
		})
	}
}

func TestNewRolesAnywhereCredentialsProvider_ExpiredCertificate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rolesanywhere-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate expired certificate
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Organization"},
			CommonName:   "expired-certificate",
		},
		NotBefore:             time.Now().Add(-48 * time.Hour), // Started 48 hours ago
		NotAfter:              time.Now().Add(-24 * time.Hour), // Expired 24 hours ago
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyBytes, _ := x509.MarshalECPrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	certPath := filepath.Join(tempDir, "expired-cert.pem")
	keyPath := filepath.Join(tempDir, "expired-key.pem")

	os.WriteFile(certPath, certPEM, 0600)
	os.WriteFile(keyPath, keyPEM, 0600)

	_, err = NewRolesAnywhereCredentialsProvider(
		certPath,
		keyPath,
		"arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/abc123",
		"arn:aws:rolesanywhere:us-east-1:123456789012:profile/def456",
		"arn:aws:iam::123456789012:role/TestRole",
		"us-east-1",
	)

	if err == nil {
		t.Error("Expected error for expired certificate, got nil")
	}
}

func TestBuildRolesAnywhereEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		region         string
		expectedScheme string
		expectedHost   string
	}{
		{
			name:           "us-east-1",
			region:         "us-east-1",
			expectedScheme: "https",
			expectedHost:   "rolesanywhere.us-east-1.amazonaws.com",
		},
		{
			name:           "eu-west-1",
			region:         "eu-west-1",
			expectedScheme: "https",
			expectedHost:   "rolesanywhere.eu-west-1.amazonaws.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, err := buildRolesAnywhereEndpoint(tt.region)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if endpoint.Scheme != tt.expectedScheme {
				t.Errorf("Scheme = %v, want %v", endpoint.Scheme, tt.expectedScheme)
			}
			if endpoint.Host != tt.expectedHost {
				t.Errorf("Host = %v, want %v", endpoint.Host, tt.expectedHost)
			}
		})
	}
}
