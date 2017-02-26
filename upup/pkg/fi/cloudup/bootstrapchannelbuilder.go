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

package cloudup

import (
	"fmt"

	channelsapi "k8s.io/kops/channels/pkg/api"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/fitasks"
	"k8s.io/kops/upup/pkg/fi/utils"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	metav1 "k8s.io/kubernetes/pkg/apis/meta/v1"
	"strings"

	"bytes"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/runtime/schema"
)

type BootstrapChannelBuilder struct {
	cluster *kops.Cluster
}

var _ fi.ModelBuilder = &BootstrapChannelBuilder{}

func (b *BootstrapChannelBuilder) Build(c *fi.ModelBuilderContext) error {
	addons, manifests, err := b.buildManifest(c)
	if err != nil {
		return err
	}
	addonsYAML, err := utils.YamlMarshal(addons)
	if err != nil {
		return fmt.Errorf("error serializing addons yaml: %v", err)
	}

	name := b.cluster.ObjectMeta.Name + "-addons-bootstrap"

	tasks := c.Tasks

	tasks[name] = &fitasks.ManagedFile{
		Name:     fi.String(name),
		Location: fi.String("addons/bootstrap-channel.yaml"),
		Contents: fi.WrapResource(fi.NewBytesResource(addonsYAML)),
	}

	for key, manifest := range manifests {
		name := b.cluster.ObjectMeta.Name + "-addons-" + key
		tasks[name] = &fitasks.ManagedFile{
			Name:     fi.String(name),
			Location: fi.String(manifest),
			Contents: &fi.ResourceHolder{Name: manifest},
		}
	}

	return nil
}

func (b *BootstrapChannelBuilder) buildManifest(c *fi.ModelBuilderContext) (*channelsapi.Addons, map[string]string, error) {
	manifests := make(map[string]string)

	addons := &channelsapi.Addons{}
	addons.Kind = "Addons"
	addons.ObjectMeta.Name = "bootstrap"

	{
		key := "core.addons.k8s.io"
		version := "1.4.0"

		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"k8s-addon": key},
			Manifest: fi.String(location),
		})
		manifests[key] = "addons/" + location
	}

	{
		key := "kube-dns.addons.k8s.io"
		version := "1.5.1"

		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"k8s-addon": key},
			Manifest: fi.String(location),
		})
		manifests[key] = "addons/" + location
	}

	{
		key := "limit-range.addons.k8s.io"
		version := "1.5.0"

		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"k8s-addon": key},
			Manifest: fi.String(location),
		})
		manifests[key] = "addons/" + location
	}

	{
		key := "dns-controller.addons.k8s.io"
		version := "1.5.3"

		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"k8s-addon": key},
			Manifest: fi.String(location),
		})

		taskName := b.cluster.ObjectMeta.Name + "-addons-" + key

		manifest, err := b.buildDNSManifest()
		if err != nil {
			return nil, nil, fmt.Errorf("error building %q: %v", key, err)
		}

		c.Tasks[taskName] = &fitasks.ManagedFile{
			Name:     fi.String(taskName),
			Location: fi.String("addons/" + location),
			Contents: fi.WrapResource(fi.NewStringResource(manifest)),
		}

	}

	{
		key := "storage-aws.addons.k8s.io"
		version := "1.5.0"

		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"k8s-addon": key},
			Manifest: fi.String(location),
		})
		manifests[key] = "addons/" + location
	}

	// The role.kubernetes.io/networking is used to label anything related to a networking addin,
	// so that if we switch networking plugins (e.g. calico -> weave or vice-versa), we'll replace the
	// old networking plugin, and there won't be old pods "floating around".

	// This means whenever we create or update a networking plugin, we should be sure that:
	// 1. the selector is role.kubernetes.io/networking=1
	// 2. every object in the manifest is labeleled with role.kubernetes.io/networking=1

	// TODO: Some way to test/enforce this?

	// TODO: Create "empty" configurations for others, so we can delete e.g. the kopeio configuration
	// if we switch to kubenet?

	if b.cluster.Spec.Networking.Kopeio != nil {
		key := "networking.kope.io"
		version := "1.0.20161116"

		// TODO: Create configuration object for cni providers (maybe create it but orphan it)?
		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"role.kubernetes.io/networking": "1"},
			Manifest: fi.String(location),
		})

		manifests[key] = "addons/" + location
	}

	if b.cluster.Spec.Networking.Weave != nil {
		key := "networking.weave"
		version := "1.9.0"

		// TODO: Create configuration object for cni providers (maybe create it but orphan it)?
		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"role.kubernetes.io/networking": "1"},
			Manifest: fi.String(location),
		})

		manifests[key] = "addons/" + location
	}

	if b.cluster.Spec.Networking.Flannel != nil {
		key := "networking.flannel"
		version := "0.7.0"

		// TODO: Create configuration object for cni providers (maybe create it but orphan it)?
		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"role.kubernetes.io/networking": "1"},
			Manifest: fi.String(location),
		})

		manifests[key] = "addons/" + location
	}

	if b.cluster.Spec.Networking.Calico != nil {
		key := "networking.projectcalico.org"
		version := "2.0.2"

		// TODO: Create configuration object for cni providers (maybe create it but orphan it)?
		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"role.kubernetes.io/networking": "1"},
			Manifest: fi.String(location),
		})

		manifests[key] = "addons/" + location
	}

	if b.cluster.Spec.Networking.Canal != nil {
		key := "networking.projectcalico.org.canal"
		version := "1.0"

		// TODO: Create configuration object for cni providers (maybe create it but orphan it)?
		location := key + "/v" + version + ".yaml"

		addons.Spec.Addons = append(addons.Spec.Addons, &channelsapi.AddonSpec{
			Name:     fi.String(key),
			Version:  fi.String(version),
			Selector: map[string]string{"role.kubernetes.io/networking": "1"},
			Manifest: fi.String(location),
		})

		manifests[key] = "addons/" + location
	}

	return addons, manifests, nil
}

func (b *BootstrapChannelBuilder) buildDNSManifest() (string, error) {
	argv, err := b.dnsControllerArgv()
	if err != nil {
		return "", err
	}

	container := v1.Container{
		Name:    "dns-controler",
		Image:   "kope/dns-controller:1.5.3",
		Command: argv,
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("50m"),
				v1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
	}

	podTemplate := v1.PodTemplateSpec{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				"k8s-addon": "dns-controller.addons.k8s.io",
				"k8s-app":   "dns-controller",
				"version":   "v1.5.3",
			},
			Annotations: map[string]string{
				"scheduler.alpha.kubernetes.io/critical-pod": "",
				"scheduler.alpha.kubernetes.io/tolerations":  "[{\"key\": \"dedicated\", \"value\": \"master\"}]",
			},
		},
		Spec: v1.PodSpec{
			NodeSelector: map[string]string{
				"kubernetes.io/role": "master",
			},
			DNSPolicy:   v1.DNSDefault, // Don't use cluster DNS (we are likely running before kube-dns)
			HostNetwork: true,
		},
	}

	// TODO: Use kube2iam here
	if fi.CloudProviderID(b.cluster.Spec.CloudProvider) == fi.CloudProviderBareMetal {
		volume := v1.Volume{
			Name: "awsdir",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/home/kops/.aws/",
				},
			},
		}
		podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes, volume)

		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			Name:      volume.Name,
			ReadOnly:  true,
			MountPath: "/root/.aws/",
		})

		//+        env:
		//+        - name: AWS_SHARED_CREDENTIALS_FILE
		//+          value: /secrets/credentials
	}

	podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, container)

	deployment := &v1beta1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      "dns-controller",
			Namespace: "kube-system",
			Labels: map[string]string{
				"k8s-addon": "dns-controller.addons.k8s.io",
				"k8s-app":   "dns-controller",
				"version":   "v1.5.3",
			},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: fi.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "dns-controller",
				},
			},
			Template: podTemplate,
		},
	}

	yaml, err := ToVersionedYaml(deployment, v1beta1.SchemeGroupVersion)
	if err != nil {
		return "", err
	}

	return string(yaml), nil
}

func (b *BootstrapChannelBuilder) dnsControllerArgv() ([]string, error) {
	var argv []string

	argv = append(argv, "/usr/bin/dns-controller")

	argv = append(argv, "--watch-ingress=false")

	switch fi.CloudProviderID(b.cluster.Spec.CloudProvider) {
	case fi.CloudProviderAWS:
		argv = append(argv, "--dns=aws-route53")
	case fi.CloudProviderGCE:
		argv = append(argv, "--dns=google-clouddns")
	case fi.CloudProviderBareMetal:
		// TODO: Make configurable
		argv = append(argv, "--dns=aws-route53")

	default:
		return nil, fmt.Errorf("unhandled cloudprovider %q", b.cluster.Spec.CloudProvider)
	}

	zone := b.cluster.Spec.DNSZone
	if zone != "" {
		if strings.Contains(zone, ".") {
			// match by name
			argv = append(argv, "--zone="+zone)
		} else {
			// match by id
			argv = append(argv, "--zone=*/"+zone)
		}
	}
	// permit wildcard updates
	argv = append(argv, "--zone=*/*")

	// Verbose, but not crazy logging
	argv = append(argv, "-v=2")

	return argv, nil
}

func encoder(gv schema.GroupVersion) runtime.Encoder {
	yaml, ok := runtime.SerializerInfoForMediaType(api.Codecs.SupportedMediaTypes(), "application/yaml")
	if !ok {
		glog.Fatalf("no YAML serializer registered")
	}
	return api.Codecs.EncoderForVersion(yaml.Serializer, gv)
}

// ToVersionedYaml encodes the object to YAML
func ToVersionedYaml(obj runtime.Object, gv schema.GroupVersion) ([]byte, error) {
	var w bytes.Buffer
	err := encoder(gv).Encode(obj, &w)
	if err != nil {
		return nil, fmt.Errorf("error encoding %T: %v", obj, err)
	}
	return w.Bytes(), nil
}
