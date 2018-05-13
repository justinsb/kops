/*
Copyright 2016 The Kubernetes Authors.

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

package k8scodecs

import (
	"bytes"
	"fmt"
	"os"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var Scheme = runtime.NewScheme()
var Codecs = serializer.NewCodecFactory(Scheme)
var ParameterCodec = runtime.NewParameterCodec(Scheme)

var Registry = registered.NewOrDie(os.Getenv("KUBE_API_VERSIONS"))
var GroupFactoryRegistry = make(announced.APIGroupFactoryRegistry)

func init() {
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	corev1.AddToScheme(Scheme)
}

func encoder(mediaType string) runtime.Encoder {
	enc, ok := runtime.SerializerInfoForMediaType(Codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		glog.Fatalf("no serializer registered for media type %q", mediaType)
	}
	gv := corev1.SchemeGroupVersion
	return Codecs.EncoderForVersion(enc.Serializer, gv)
}

// ToVersionedYaml encodes the object to YAML
func ToVersionedYaml(obj runtime.Object) ([]byte, error) {
	return Serialize(obj, "application/yaml")
}

// ToVersionedJSON encodes the object to JSON
func ToVersionedJSON(obj runtime.Object) ([]byte, error) {
	return Serialize(obj, "application/json")
}

// Serialize encodes the object to the specified mediaType (application/json or application/yaml)
func Serialize(obj runtime.Object, mediaType string) ([]byte, error) {
	var w bytes.Buffer
	err := encoder(mediaType).Encode(obj, &w)
	if err != nil {
		return nil, fmt.Errorf("error encoding %T: %v", obj, err)
	}
	return w.Bytes(), nil
}
