package addon

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kops/pkg/addon/channel"
	"k8s.io/kops/pkg/addon/kustomize"
	"k8s.io/kops/pkg/k8scodecs"
	"k8s.io/kops/util/pkg/vfs"
	"k8s.io/kubectl/pkg/kustomize/resource"
	"k8s.io/kubectl/pkg/kustomize/types"
	kutil "k8s.io/kubectl/pkg/kustomize/util"
	"k8s.io/kubectl/pkg/kustomize/util/fs"
	"k8s.io/kubectl/pkg/loader"
)

type Bundle struct {
	configMap *corev1.ConfigMap
	patches   map[string]runtime.Object
	selector  map[string]string
	resources resource.ResourceCollection
}

func (b *Bundle) HasObjectMatchingSelector(selector map[string]string) bool {
	for _, o := range b.resources {
		labels := o.Data.GetLabels()
		matches := true
		for k, v := range selector {
			if labels[k] != v {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func (b *Bundle) AddPatch(key string, patch runtime.Object) {
	if b.patches == nil {
		b.patches = make(map[string]runtime.Object)
	}
	b.patches[key] = patch
}

// Name is the unique identifier for the bundle
func (b *Bundle) Name() string {
	return b.configMap.Name
}

// Version is the currently selected version
func (b *Bundle) Version() string {
	version := b.get("_system/current")
	if version == "" {
		glog.Errorf("bundle did not have version: %v", b)
	}
	return version
}

// Selector determines the objects we should replace (prune)
func (b *Bundle) Selector() map[string]string {
	return b.selector
}

func Parse(configMap *corev1.ConfigMap, key string) (*Bundle, error) {
	b := &Bundle{configMap: configMap}
	// TODO: Validation

	b.selector = map[string]string{
		"k8s-addon": key,
	}

	resources, err := b.buildResources()
	if err != nil {
		return nil, err
	}

	b.resources = resources

	return b, nil
}

func Load(channelURL string) (*Bundle, error) {
	channelPath, err := vfs.Context.BuildVfsPath(channelURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing channel path %s: %v", channelURL, err)
	}

	c, err := channel.LoadAddonChannel(channelPath)
	if err != nil {
		return nil, err
	}

	criteria := &channel.Criteria{}
	addon := c.BestMatch(criteria)
	if addon == nil {
		return nil, fmt.Errorf("unable to find addon %v in %s", criteria, channelPath)
	}

	lastSlash := strings.LastIndex(channelURL, "/")
	if lastSlash == -1 {
		return nil, fmt.Errorf("cannot determine parent directory of %s", channelURL)
	}
	channelDir := channelURL[:lastSlash]
	channelDirPath, err := vfs.Context.BuildVfsPath(channelDir)
	if err != nil {
		return nil, fmt.Errorf("error parsing channel directory %s: %v", channelDir, err)
	}

	addonPath := channelDirPath.Join(addon.Path)

	l := kustomize.NewHttpLoader()

	configMap, err := createConfigMap(channelURL, addonPath.Path(), l)
	if err != nil {
		return nil, err
	}

	return &Bundle{configMap: configMap}, nil
}

func (a *Bundle) Persist() *corev1.ConfigMap {
	return a.configMap.DeepCopy()
}

func (a *Bundle) Update(new *Bundle) *Bundle {
	m := &corev1.ConfigMap{}
	m.Data = map[string]string{}
	for k, v := range a.configMap.Data {
		if strings.HasPrefix(k, "_system/base/") {
			continue
		}
		m.Data[k] = v
	}
	for k, v := range new.configMap.Data {
		m.Data[k] = v
	}
	return &Bundle{configMap: m}
}

func createConfigMap(channelURL string, base string, l loader.SchemeLoader) (*corev1.ConfigMap, error) {
	config := &corev1.ConfigMap{}
	config.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	config.Data = map[string]string{
		"_system/current": base,
		"_system/channel": channelURL,
	}

	files, err := kustomize.Walk(base, l)
	if err != nil {
		return nil, fmt.Errorf("unexpected error from Build: %v", err)
	}

	for _, f := range files {
		if !strings.HasPrefix(f.Path, base) {
			// We could support this, but we probably don't want to
			return nil, fmt.Errorf("unhandled file in parent directory %q", f.Path)
		}

		relativePath := strings.TrimPrefix(f.Path, base)
		config.Data["_system/base/"+relativePath] = string(f.Contents)
	}

	return config, nil
}

// Manifest returns the fully rendered manifest, ready to kubectl apply
func (b *Bundle) Manifest() ([]byte, error) {
	resources, err := b.buildResources()
	if err != nil {
		return nil, err
	}

	// Output the objects.
	res, err := kutil.Encode(resources)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (b *Bundle) buildResources() (resource.ResourceCollection, error) {
	ffs := fs.MakeFakeFS()
	for k, v := range b.configMap.Data {
		if strings.HasPrefix(k, "_system/kops/") {
			return nil, fmt.Errorf("_system/kops/ tree is reserved and should not be specified")
		}
		ffs.WriteFile("/"+k, []byte(v))
	}

	kustomization := b.get("kustomization.yaml")
	if kustomization == "" {
		o := b.buildDefaultKustomization("")
		o.Bases = []string{"_system/kops/"}
		b, err := yaml.Marshal(o)
		if err != nil {
			return nil, fmt.Errorf("error serializing kustomization: %v", err)
		}

		ffs.WriteFile("/kustomization.yaml", b)
	}

	{
		o := &types.Kustomization{}
		var patchKeys []string
		for k := range b.patches {
			patchKeys = append(patchKeys, k)
		}
		sort.Strings(patchKeys)

		for _, k := range patchKeys {
			patch := b.patches[k]
			patchBytes, err := k8scodecs.ToVersionedYaml(patch)
			if err != nil {
				return nil, fmt.Errorf("error serializing %v: %v", patch, err)
			}

			glog.Infof("serialized patch %s", string(patchBytes))
			o.Patches = append(o.Patches, "patches/"+k+".yaml")
			ffs.WriteFile("/"+"_system/kops/patches/"+k+".yaml", patchBytes)
		}

		o.Bases = []string{"../base/"}
		o.CommonLabels = b.selector
		b, err := yaml.Marshal(o)
		if err != nil {
			return nil, fmt.Errorf("error serializing kustomization: %v", err)
		}

		ffs.WriteFile("/_system/kops/kustomization.yaml", b)
	}

	return kustomize.BuildResources("/", loader.NewFileLoader(ffs))
}

func (b *Bundle) get(k string) string {
	v := b.configMap.Data[k]
	v = strings.TrimSpace(v)
	return v
}

func (b *Bundle) buildDefaultKustomization(base string) *types.Kustomization {
	var keys []string
	for k := range b.configMap.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	o := &types.Kustomization{}

	for _, k := range keys {
		if strings.HasPrefix(k, base+"resources/") {
			o.Resources = append(o.Resources, strings.TrimPrefix(base, k))
		}
		if strings.HasPrefix(k, base+"patches/") {
			o.Patches = append(o.Patches, strings.TrimPrefix(base, k))
		}
	}

	return o
}
