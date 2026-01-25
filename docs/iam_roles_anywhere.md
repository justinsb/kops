# IAM Roles Anywhere for S3 State Store

kops supports AWS IAM Roles Anywhere for S3 state store authentication, allowing you to use X.509 certificates instead of traditional AWS access keys or instance profiles.

## Overview

IAM Roles Anywhere enables workloads running outside of AWS (or without IAM roles) to obtain temporary AWS credentials using X.509 certificates. This is particularly useful for:

- On-premises workloads accessing AWS S3
- Multi-cloud environments
- CI/CD systems that need AWS access
- Environments where you already have a PKI infrastructure

## Prerequisites

Before using IAM Roles Anywhere with kops, you need to:

1. **Set up IAM Roles Anywhere in AWS:**
   - Create a Trust Anchor pointing to your Certificate Authority (CA)
   - Create a Profile that maps to an IAM role
   - Ensure the IAM role has appropriate S3 permissions for your kops state store

2. **Obtain a client certificate:**
   - Generate a certificate signed by your CA
   - Ensure the certificate is valid and not expired
   - Keep the private key secure

## AWS Setup with CLI

This section provides step-by-step AWS CLI commands to set up IAM Roles Anywhere for kops.

### Step 1: Create or Prepare Your Certificate Authority (CA)

First, you need a CA certificate. You can either use an existing CA or create a new self-signed CA for testing.

#### Option A: Create a Self-Signed CA (for testing)

```bash
# Generate CA private key
openssl genrsa -out ca-key.pem 4096

# Create CA certificate (valid for 10 years)
openssl req -new -x509 -days 3650 -key ca-key.pem -out ca-cert.pem \
  -subj "/C=US/ST=State/L=City/O=Organization/OU=IT/CN=Test CA"
```

#### Option B: Use Your Existing Enterprise CA

If you have an existing CA, export the CA certificate in PEM format:

```bash
# Example for exporting from a file
cat your-ca.crt > ca-cert.pem
```

### Step 2: Create the Trust Anchor

Upload your CA certificate to IAM Roles Anywhere as a Trust Anchor:

```bash
# Create the trust anchor
aws rolesanywhere create-trust-anchor \
  --name "KopsTrustAnchor" \
  --source "sourceType=CERTIFICATE_BUNDLE,sourceData={x509CertificateData=$(cat ca-cert.pem | base64)}" \
  --enabled \
  --region us-east-1

# Save the Trust Anchor ARN from the output
# Example output: arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/a1b2c3d4-e5f6-g7h8-i9j0-k1l2m3n4o5p6
```

**Note:** Save the Trust Anchor ARN from the output - you'll need it later.

### Step 3: Create the IAM Role

Create an IAM role with permissions to access your kops S3 state store:

```bash
# Create a trust policy document for IAM Roles Anywhere
cat > trust-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "rolesanywhere.amazonaws.com"
      },
      "Action": [
        "sts:AssumeRole",
        "sts:SetSourceIdentity",
        "sts:TagSession"
      ],
      "Condition": {
        "ArnEquals": {
          "aws:SourceArn": "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/YOUR_TRUST_ANCHOR_ID"
        }
      }
    }
  ]
}
EOF

# Replace YOUR_TRUST_ANCHOR_ID with your actual Trust Anchor ARN from Step 2

# Create the IAM role
aws iam create-role \
  --role-name KopsS3Access \
  --assume-role-policy-document file://trust-policy.json \
  --description "Role for kops to access S3 state store via IAM Roles Anywhere"

# Save the Role ARN from the output
# Example output: arn:aws:iam::123456789012:role/KopsS3Access
```

### Step 4: Attach S3 Permissions to the Role

Create and attach a policy that grants access to your kops S3 state store:

```bash
# Create an S3 access policy
cat > s3-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::YOUR-KOPS-STATE-BUCKET/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket",
        "s3:GetBucketLocation",
        "s3:GetBucketVersioning"
      ],
      "Resource": "arn:aws:s3:::YOUR-KOPS-STATE-BUCKET"
    }
  ]
}
EOF

# Replace YOUR-KOPS-STATE-BUCKET with your actual bucket name

# Create the policy
aws iam create-policy \
  --policy-name KopsS3StateStoreAccess \
  --policy-document file://s3-policy.json \
  --description "Grants access to kops S3 state store"

# Attach the policy to the role
aws iam attach-role-policy \
  --role-name KopsS3Access \
  --policy-arn arn:aws:iam::123456789012:policy/KopsS3StateStoreAccess

# Note: Replace 123456789012 with your AWS account ID
```

### Step 5: Create the Profile

Create an IAM Roles Anywhere Profile that maps to your IAM role:

```bash
# Create the profile
aws rolesanywhere create-profile \
  --name "KopsProfile" \
  --role-arns "arn:aws:iam::123456789012:role/KopsS3Access" \
  --enabled \
  --duration-seconds 3600 \
  --region us-east-1

# Save the Profile ARN from the output
# Example output: arn:aws:rolesanywhere:us-east-1:123456789012:profile/b2c3d4e5-f6g7-h8i9-j0k1-l2m3n4o5p6q7
```

**Note:** Save the Profile ARN from the output - you'll need it for kops configuration.

### Step 6: Generate Client Certificate

Generate a client certificate signed by your CA:

```bash
# Generate client private key
openssl genrsa -out client-key.pem 2048

# Create certificate signing request (CSR)
openssl req -new -key client-key.pem -out client.csr \
  -subj "/C=US/ST=State/L=City/O=Organization/OU=Kops/CN=kops-client"

# Sign the client certificate with your CA (valid for 1 year)
openssl x509 -req -in client.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -CAcreateserial -out client-cert.pem -days 365 \
  -extensions v3_end -extfile <(cat <<EOF
[v3_end]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF
)

# Secure the private key
chmod 600 client-key.pem

# Clean up CSR
rm client.csr
```

### Step 7: Verify the Setup

Verify all components are created correctly:

```bash
# Verify Trust Anchor
aws rolesanywhere get-trust-anchor \
  --trust-anchor-id YOUR_TRUST_ANCHOR_ID \
  --region us-east-1

# Verify Profile
aws rolesanywhere get-profile \
  --profile-id YOUR_PROFILE_ID \
  --region us-east-1

# Verify IAM Role
aws iam get-role --role-name KopsS3Access

# Verify IAM Role Policies
aws iam list-attached-role-policies --role-name KopsS3Access
```

### Step 8: Set Environment Variables

Export the environment variables for kops to use:

```bash
export AWS_ROLES_ANYWHERE_CERT_PATH="$(pwd)/client-cert.pem"
export AWS_ROLES_ANYWHERE_KEY_PATH="$(pwd)/client-key.pem"
export AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN="arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/YOUR_TRUST_ANCHOR_ID"
export AWS_ROLES_ANYWHERE_PROFILE_ARN="arn:aws:rolesanywhere:us-east-1:123456789012:profile/YOUR_PROFILE_ID"
export AWS_ROLES_ANYWHERE_ROLE_ARN="arn:aws:iam::123456789012:role/KopsS3Access"
export AWS_ROLES_ANYWHERE_REGION="us-east-1"
```

### Step 9: Test with kops

Test the configuration with a simple kops command:

```bash
# Test listing clusters
kops get cluster --state=s3://YOUR-KOPS-STATE-BUCKET

# If successful, you should see your clusters or an empty list
```

### Troubleshooting Setup

If you encounter issues during setup:

**Trust Anchor Issues:**
```bash
# List all trust anchors
aws rolesanywhere list-trust-anchors --region us-east-1

# Enable a trust anchor if disabled
aws rolesanywhere enable-trust-anchor \
  --trust-anchor-id YOUR_TRUST_ANCHOR_ID \
  --region us-east-1
```

**Profile Issues:**
```bash
# List all profiles
aws rolesanywhere list-profiles --region us-east-1

# Enable a profile if disabled
aws rolesanywhere enable-profile \
  --profile-id YOUR_PROFILE_ID \
  --region us-east-1
```

**Role Trust Policy Issues:**
```bash
# Update the trust policy if needed
aws iam update-assume-role-policy \
  --role-name KopsS3Access \
  --policy-document file://trust-policy.json
```

### Clean Up (Optional)

To remove the IAM Roles Anywhere setup:

```bash
# Disable and delete the profile
aws rolesanywhere disable-profile --profile-id YOUR_PROFILE_ID --region us-east-1
aws rolesanywhere delete-profile --profile-id YOUR_PROFILE_ID --region us-east-1

# Disable and delete the trust anchor
aws rolesanywhere disable-trust-anchor --trust-anchor-id YOUR_TRUST_ANCHOR_ID --region us-east-1
aws rolesanywhere delete-trust-anchor --trust-anchor-id YOUR_TRUST_ANCHOR_ID --region us-east-1

# Detach and delete the policy
aws iam detach-role-policy \
  --role-name KopsS3Access \
  --policy-arn arn:aws:iam::123456789012:policy/KopsS3StateStoreAccess
aws iam delete-policy --policy-arn arn:aws:iam::123456789012:policy/KopsS3StateStoreAccess

# Delete the role
aws iam delete-role --role-name KopsS3Access
```

## Configuration

To configure kops to use IAM Roles Anywhere for S3 authentication, set the following environment variables:

### Required Environment Variables

| Environment Variable | Description | Example |
|---------------------|-------------|---------|
| `AWS_ROLES_ANYWHERE_CERT_PATH` | Path to your X.509 certificate file | `/path/to/client-cert.pem` |
| `AWS_ROLES_ANYWHERE_KEY_PATH` | Path to your private key file | `/path/to/client-key.pem` |
| `AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN` | ARN of your IAM Roles Anywhere Trust Anchor | `arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/a1b2c3d4` |
| `AWS_ROLES_ANYWHERE_PROFILE_ARN` | ARN of your IAM Roles Anywhere Profile | `arn:aws:rolesanywhere:us-east-1:123456789012:profile/e5f6g7h8` |
| `AWS_ROLES_ANYWHERE_ROLE_ARN` | ARN of the IAM role to assume | `arn:aws:iam::123456789012:role/KopsStateAccess` |

### Optional Environment Variables

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `AWS_ROLES_ANYWHERE_REGION` | AWS region for the IAM Roles Anywhere service | `us-east-1` |

## Usage Example

### Basic Setup

```bash
# Export environment variables
export AWS_ROLES_ANYWHERE_CERT_PATH="/home/user/.aws/certificates/client-cert.pem"
export AWS_ROLES_ANYWHERE_KEY_PATH="/home/user/.aws/certificates/client-key.pem"
export AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN="arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/abc123def456"
export AWS_ROLES_ANYWHERE_PROFILE_ARN="arn:aws:rolesanywhere:us-east-1:123456789012:profile/ghi789jkl012"
export AWS_ROLES_ANYWHERE_ROLE_ARN="arn:aws:iam::123456789012:role/KopsS3Access"
export AWS_ROLES_ANYWHERE_REGION="us-east-1"

# Use kops as normal
kops get cluster --state=s3://my-kops-state-store
```

### CI/CD Pipeline Example

```yaml
# Example GitHub Actions workflow
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Configure IAM Roles Anywhere
        env:
          AWS_ROLES_ANYWHERE_CERT_PATH: ${{ secrets.AWS_CERT_PATH }}
          AWS_ROLES_ANYWHERE_KEY_PATH: ${{ secrets.AWS_KEY_PATH }}
          AWS_ROLES_ANYWHERE_TRUST_ANCHOR_ARN: ${{ secrets.AWS_TRUST_ANCHOR_ARN }}
          AWS_ROLES_ANYWHERE_PROFILE_ARN: ${{ secrets.AWS_PROFILE_ARN }}
          AWS_ROLES_ANYWHERE_ROLE_ARN: ${{ secrets.AWS_ROLE_ARN }}
        run: |
          echo "${{ secrets.AWS_CERT }}" > /tmp/cert.pem
          echo "${{ secrets.AWS_KEY }}" > /tmp/key.pem
          chmod 600 /tmp/key.pem
          kops get cluster --state=s3://my-kops-state-store
```

## IAM Policy Requirements

The IAM role specified in `AWS_ROLES_ANYWHERE_ROLE_ARN` must have appropriate permissions for your kops state store. Example policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::my-kops-state-store",
        "arn:aws:s3:::my-kops-state-store/*"
      ]
    }
  ]
}
```

The trust policy for the IAM role must allow IAM Roles Anywhere to assume it:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "rolesanywhere.amazonaws.com"
      },
      "Action": [
        "sts:AssumeRole",
        "sts:SetSourceIdentity",
        "sts:TagSession"
      ],
      "Condition": {
        "ArnEquals": {
          "aws:SourceArn": [
            "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/abc123def456"
          ]
        }
      }
    }
  ]
}
```

## Certificate Requirements

Your X.509 certificate must meet the following requirements:

- **Valid**: The certificate must be within its validity period (NotBefore < now < NotAfter)
- **Signed by trusted CA**: The certificate must be signed by the CA associated with your Trust Anchor
- **Client authentication**: The certificate should have the Extended Key Usage for client authentication
- **Key type**: RSA (2048-bit or higher) or ECDSA (P-256 or P-384) keys are supported

## Credential Caching

kops automatically caches IAM Roles Anywhere credentials to minimize API calls. Credentials are:

- Cached until 5 minutes before expiration
- Automatically refreshed when expired
- Valid for 1 hour by default (3600 seconds)

## Troubleshooting

### Certificate Validation Errors

If you see errors about certificate validation:

```
failed to load certificate and key: tls: failed to find any PEM data
```

Ensure your certificate and key files are in PEM format and readable.

### Expired Certificate

```
certificate has expired (NotAfter: 2024-01-01T00:00:00Z)
```

Your certificate has expired. Generate a new certificate from your CA.

### Permission Denied

```
CreateSession API failed (status 403): AccessDeniedException
```

Check that:
- Your certificate is signed by the CA associated with the Trust Anchor
- The Trust Anchor is enabled
- The Profile is enabled and correctly configured
- The IAM role trust policy allows IAM Roles Anywhere

### Wrong Region

If you're using a regional endpoint, ensure `AWS_ROLES_ANYWHERE_REGION` matches the region where your Trust Anchor and Profile are configured.

## Security Best Practices

1. **Protect Private Keys**: Store private keys securely with restricted permissions (chmod 600)
2. **Certificate Rotation**: Regularly rotate certificates before they expire
3. **Least Privilege**: Grant only necessary S3 permissions to the IAM role
4. **Audit Logging**: Enable CloudTrail logging for IAM Roles Anywhere API calls
5. **Certificate Revocation**: Use Certificate Revocation Lists (CRLs) to revoke compromised certificates

## How It Works

When IAM Roles Anywhere is configured:

1. kops detects the environment variables on startup
2. When accessing S3, kops establishes a mutual TLS (mTLS) connection to the IAM Roles Anywhere service
3. The client certificate is presented during the TLS handshake
4. IAM Roles Anywhere validates the certificate against the Trust Anchor
5. Temporary AWS credentials are returned (access key, secret key, session token)
6. These credentials are used to access the S3 state store
7. Credentials are cached and automatically refreshed before expiration

## Alternatives

If IAM Roles Anywhere doesn't fit your use case, kops also supports:

- **IAM Instance Profiles**: For EC2 instances
- **Static Credentials**: Via `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
- **Shared Credentials File**: Via `~/.aws/credentials`
- **Web Identity Token**: For Kubernetes service accounts (IRSA)
- **Custom S3 Endpoints**: Via `S3_ENDPOINT` for S3-compatible storage

## References

- [AWS IAM Roles Anywhere Documentation](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/introduction.html)
- [AWS IAM Roles Anywhere API Reference](https://docs.aws.amazon.com/rolesanywhere/latest/APIReference/Welcome.html)
- [kops State Store Documentation](https://kops.sigs.k8s.io/state/)
