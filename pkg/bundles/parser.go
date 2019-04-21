package bundles

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
)

func ParseToTypedObjects(component *Component, scheme *runtime.Scheme) ([]runtime.Object, error) {
	pretty := false
	jsonSerializer := json.NewSerializer(json.DefaultMetaFactory, scheme, scheme, pretty)

	var objects []runtime.Object
	for _, obj := range component.Spec.Objects {
		j, err := obj.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("error marshalling unstructured to JSON: %v", err)
		}

		obj, _, err := jsonSerializer.Decode(j, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("error parsing json: %v", err)
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// ParseBytes parses a set of objects from a []byte
func ParseBytes(data []byte, yamlDecoder runtime.Decoder) ([]runtime.Object, error) {
	//yamlDecoder := yaml.NewDecodingSerializer(serializer)

	reader := json.YAMLFramer.NewFrameReader(ioutil.NopCloser(bytes.NewReader([]byte(data))))
	d := streaming.NewDecoder(reader, yamlDecoder)

	var objects []runtime.Object
	for {
		obj, _, err := d.Decode(nil, nil)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error during parse: %v", err)
		}
		objects = append(objects, obj)
	}

	return objects, nil
}
