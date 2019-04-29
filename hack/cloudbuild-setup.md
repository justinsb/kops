Based on [this](https://cloud.google.com/cloud-build/docs/securing-builds/use-encrypted-secrets-credentials)

To create a KeyRing, run the following command in your shell or terminal window:

PROJECT_NAME=`gcloud config get-value project`
PROJECT_ID=`gcloud projects describe ${PROJECT_NAME} --format=json | jq -r .projectNumber`

KEY_NAME=kops
KEYRING_NAME=kops

```bash

gcloud kms keyrings create ${KEYRING_NAME} \
  --location=global

```

  To create a CryptoKey:

```bash
  
gcloud kms keys create ${KEY_NAME} --location=global --keyring=${KEYRING_NAME} --purpose=encryption

```

```bash

gcloud kms keys add-iam-policy-binding ${KEY_NAME} --location=global --keyring=${KEYRING_NAME} \
--member=serviceAccount:${PROJECT_ID}@cloudbuild.gserviceaccount.com --role=roles/cloudkms.cryptoKeyEncrypterDecrypter
```


```bash
gcloud kms encrypt --plaintext-file=/tmp/docker.json --ciphertext-file=- --location=global --keyring=kops --key=kops | base64 -w 0

  ```
