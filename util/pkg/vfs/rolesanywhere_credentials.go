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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"k8s.io/klog/v2"
)

// RolesAnywhereCredentialsProvider implements AWS credentials retrieval using IAM Roles Anywhere
// with X.509 certificate-based authentication
type RolesAnywhereCredentialsProvider struct {
	certPath        string
	keyPath         string
	trustAnchorARN  string
	profileARN      string
	roleARN         string
	region          string
	sessionDuration int32

	// cached credentials
	cachedCreds aws.Credentials
	cachedUntil time.Time
	httpClient  *http.Client
}

// rolesAnywhereConfig holds the configuration for IAM Roles Anywhere from environment variables
type rolesAnywhereConfig struct {
	certPath       string
	keyPath        string
	trustAnchorARN string
	profileARN     string
	roleARN        string
	region         string
}

// getRolesAnywhereConfig reads IAM Roles Anywhere configuration from environment variables
// Returns nil if any required environment variable is not set
func getRolesAnywhereConfig() *rolesAnywhereConfig {
	certPath := os.Getenv("AWS_ROLES_ANYWHERE_CERT_PATH")
	keyPath := os.Getenv("AWS_ROLES_ANYWHERE_KEY_PATH")
	trustAnchorARN := os.Getenv("AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN")
	profileARN := os.Getenv("AWS_ROLES_ANYWHERE_PROFILE_ARN")
	roleARN := os.Getenv("AWS_ROLES_ANYWHERE_ROLE_ARN")

	// If any required field is missing, return nil
	if certPath == "" || keyPath == "" || trustAnchorARN == "" || profileARN == "" || roleARN == "" {
		return nil
	}

	region := os.Getenv("AWS_ROLES_ANYWHERE_REGION")
	if region == "" {
		region = "us-east-1"
	}

	return &rolesAnywhereConfig{
		certPath:       certPath,
		keyPath:        keyPath,
		trustAnchorARN: trustAnchorARN,
		profileARN:     profileARN,
		roleARN:        roleARN,
		region:         region,
	}
}

// NewRolesAnywhereCredentialsProvider creates a new RolesAnywhereCredentialsProvider
func NewRolesAnywhereCredentialsProvider(certPath, keyPath, trustAnchorARN, profileARN, roleARN, region string) (*RolesAnywhereCredentialsProvider, error) {
	// Load certificate and private key to validate they exist and are valid
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate and key: %w", err)
	}

	// Verify the certificate is valid
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check if certificate is expired
	now := time.Now()
	if now.Before(x509Cert.NotBefore) {
		return nil, fmt.Errorf("certificate is not yet valid (NotBefore: %v)", x509Cert.NotBefore)
	}
	if now.After(x509Cert.NotAfter) {
		return nil, fmt.Errorf("certificate has expired (NotAfter: %v)", x509Cert.NotAfter)
	}

	klog.V(4).Infof("IAM Roles Anywhere certificate loaded: Subject=%s, Expires=%v", x509Cert.Subject, x509Cert.NotAfter)

	// Create HTTP client with the certificate for mTLS
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			},
		},
		Timeout: 30 * time.Second,
	}

	return &RolesAnywhereCredentialsProvider{
		certPath:        certPath,
		keyPath:         keyPath,
		trustAnchorARN:  trustAnchorARN,
		profileARN:      profileARN,
		roleARN:         roleARN,
		region:          region,
		sessionDuration: 3600, // 1 hour
		httpClient:      httpClient,
	}, nil
}

// Retrieve implements aws.CredentialsProvider
func (p *RolesAnywhereCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	// Check if we have valid cached credentials
	if !p.cachedCreds.Expired() && time.Now().Before(p.cachedUntil) {
		klog.V(8).Info("Using cached IAM Roles Anywhere credentials")
		return p.cachedCreds, nil
	}

	klog.V(4).Info("Retrieving new IAM Roles Anywhere credentials")

	// Build the CreateSession API endpoint
	endpoint := fmt.Sprintf("https://rolesanywhere.%s.amazonaws.com/sessions", p.region)

	// Build request body according to AWS IAM Roles Anywhere API specification
	// https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/API_CreateSession.html
	requestBody := map[string]interface{}{
		"durationSeconds": p.sessionDuration,
		"profileArn":      p.profileARN,
		"roleArn":         p.roleARN,
		"trustAnchorArn":  p.trustAnchorARN,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	klog.V(8).Infof("Calling IAM Roles Anywhere CreateSession API: endpoint=%s, role=%s", endpoint, p.roleARN)

	// Make the request with mTLS authentication
	// The certificate is automatically presented during TLS handshake
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to call CreateSession API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Try to parse error response
		var errorResp struct {
			Message string `json:"message"`
			Type    string `json:"__type"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil {
			return aws.Credentials{}, fmt.Errorf("CreateSession API failed (status %d): %s - %s", resp.StatusCode, errorResp.Type, errorResp.Message)
		}
		return aws.Credentials{}, fmt.Errorf("CreateSession API failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse successful response
	// Response format: https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/API_CreateSession.html
	var sessionResponse struct {
		CredentialSet []struct {
			Credentials struct {
				AccessKeyId     string    `json:"accessKeyId"`
				SecretAccessKey string    `json:"secretAccessKey"`
				SessionToken    string    `json:"sessionToken"`
				Expiration      time.Time `json:"expiration"`
			} `json:"credentials"`
			AssumedRoleUser struct {
				Arn           string `json:"arn"`
				AssumedRoleId string `json:"assumedRoleId"`
			} `json:"assumedRoleUser"`
			PackedPolicySize int    `json:"packedPolicySize"`
			RoleArn          string `json:"roleArn"`
			SourceIdentity   string `json:"sourceIdentity,omitempty"`
		} `json:"credentialSet"`
		SubjectArn string `json:"subjectArn"`
	}

	if err := json.Unmarshal(body, &sessionResponse); err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to parse CreateSession response: %w (body: %s)", err, string(body))
	}

	if len(sessionResponse.CredentialSet) == 0 {
		return aws.Credentials{}, fmt.Errorf("no credentials returned in CreateSession response")
	}

	creds := sessionResponse.CredentialSet[0].Credentials
	expiration := creds.Expiration

	// Validate we got all required fields
	if creds.AccessKeyId == "" || creds.SecretAccessKey == "" || creds.SessionToken == "" {
		return aws.Credentials{}, fmt.Errorf("incomplete credentials in response")
	}

	// Cache the credentials
	p.cachedCreds = aws.Credentials{
		AccessKeyID:     creds.AccessKeyId,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Source:          "IAMRolesAnywhere",
		CanExpire:       true,
		Expires:         expiration,
	}
	// Refresh 5 minutes before expiration to avoid using expired credentials
	p.cachedUntil = expiration.Add(-5 * time.Minute)

	klog.V(4).Infof("Successfully retrieved IAM Roles Anywhere credentials (subject=%s, expires=%v)", sessionResponse.SubjectArn, expiration)

	return p.cachedCreds, nil
}

// isRolesAnywhereConfigured checks if IAM Roles Anywhere environment variables are configured
func isRolesAnywhereConfigured() bool {
	return getRolesAnywhereConfig() != nil
}

// buildRolesAnywhereEndpoint constructs the IAM Roles Anywhere API endpoint for a given region
func buildRolesAnywhereEndpoint(region string) (*url.URL, error) {
	endpoint := fmt.Sprintf("https://rolesanywhere.%s.amazonaws.com", region)
	return url.Parse(endpoint)
}
