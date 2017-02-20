package gce

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
)

// The tag name we use to differentiate multiple logically independent clusters running in the same region
const GceLabelNameKubernetesCluster = "k8s-io-cluster-name"

const GceLabelNameRolePrefix = "k8s-io-role-"
const GceLabelNameEtcdClusterPrefix = "k8s-io-etcd-"

// EncodeGCELabel encodes a string into an RFC1035 compatible value, suitable for use as GCE label key or value
// We use a URI inspired escaping, but with - instead of %.
func EncodeGCELabel(s string) string {
	var b bytes.Buffer

	for i := 0; i < len(s); i++ {
		c := s[i]
		if ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') {
			b.WriteByte(c)
		} else {
			b.WriteByte('-')
			b.WriteByte("0123456789abcdef"[c>>4])
			b.WriteByte("0123456789abcdef"[c&15])
		}
	}

	return b.String()
}

// DecodeGCELabel reverse EncodeGCELabel, taking the encoded RFC1035 compatible value back to a string
func DecodeGCELabel(s string) (string, error) {
	uriForm := strings.Replace(s, "-", "%", -1)
	v, err := url.QueryUnescape(uriForm)
	if err != nil {
		return "", fmt.Errorf("Cannot decode GCE label: %q", s)
	}
	return v, nil
}
