module k8s.io/kops

go 1.12

// Version kubernetes-1.15.3
//replace k8s.io/kubernetes => k8s.io/kubernetes v1.15.3
//replace k8s.io/api => k8s.io/api kubernetes-1.15.3
//replace k8s.io/apimachinery => k8s.io/apimachinery kubernetes-1.15.3
//replace k8s.io/client-go => k8s.io/client-go kubernetes-1.15.3
//replace k8s.io/cloud-provider => k8s.io/cloud-provider kubernetes-1.15.3
//replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers kubernetes-1.15.3

replace k8s.io/kubernetes => k8s.io/kubernetes v1.15.3

replace k8s.io/api => k8s.io/api v0.0.0-20190819141258-3544db3b9e44

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190819141724-e14f31a72a77

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190819145148-d91c85d212d5

// Dependencies we don't really need, except that kubernetes specifies them as v0.0.0 which confuses go.mod
//replace k8s.io/apiserver => k8s.io/apiserver kubernetes-1.15.3
//replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver kubernetes-1.15.3
//replace k8s.io/kube-scheduler => k8s.io/kube-scheduler kubernetes-1.15.3
//replace k8s.io/kube-proxy => k8s.io/kube-proxy kubernetes-1.15.3
//replace k8s.io/cri-api => k8s.io/cri-api kubernetes-1.15.3
//replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib kubernetes-1.15.3
//replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers kubernetes-1.15.3
//replace k8s.io/component-base => k8s.io/component-base kubernetes-1.15.3
//replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap kubernetes-1.15.3
//replace k8s.io/metrics => k8s.io/metrics kubernetes-1.15.3
//replace k8s.io/sample-apiserver => k8s.io/sample-apiserver kubernetes-1.15.3
//replace k8s.io/kube-aggregator => k8s.io/kube-aggregator kubernetes-1.15.3
//replace k8s.io/kubelet => k8s.io/kubelet kubernetes-1.15.3
//replace k8s.io/cli-runtime => k8s.io/cli-runtime kubernetes-1.15.3
//replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager kubernetes-1.15.3
//replace k8s.io/code-generator => k8s.io/code-generator kubernetes-1.15.3

replace k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190819142446-92cc630367d0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190819143637-0dbe462fe92d

replace k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190819144524-827174bad5e8

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190819144027-541433d7ce35

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190819144832-f53437941eef

replace k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190612205613-18da4a14b22b

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190819144657-d1a724e0828e

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190819144346-2e47de1df0f0

replace k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190817025403-3ae76f584e79

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190819145328-4831a4ced492

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190819145509-592c9a46fd00

replace k8s.io/component-base => k8s.io/component-base v0.0.0-20190819141909-f0f7c184477d

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190819145008-029dd04813af

replace k8s.io/metrics => k8s.io/metrics v0.0.0-20190819143841-305e1cef1ab1

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190819143045-c84c31c165c4

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190819142756-13daafd3604f

require (
	cloud.google.com/go v0.46.3
	github.com/Azure/go-autorest v13.0.1+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.9.2 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.7.0 // indirect
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd
	github.com/Masterminds/semver v1.3.1 // indirect
	github.com/Masterminds/sprig v2.17.1+incompatible
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/ajstarks/svgo v0.0.0-20190826172357-de52242f3d65 // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/aokoli/goutils v1.0.1 // indirect
	github.com/aws/aws-sdk-go v1.23.0
	github.com/bazelbuild/bazel-gazelle v0.18.2-0.20190823151146-67c9ddf12d8a
	github.com/blang/semver v3.5.1+incompatible
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/client9/misspell v0.3.4
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.15+incompatible
	github.com/coreos/go-oidc v2.1.0+incompatible // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/creack/pty v1.1.9 // indirect
	github.com/denverdino/aliyungo v0.0.0-20180316152028-2581e433b270
	github.com/digitalocean/godo v1.19.0
	github.com/docker/engine-api v0.0.0-20160509170047-dea108d3aa0c
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20190911111923-ecfe977594f1 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190911111923-ecfe977594f1 // indirect
	github.com/emicklei/go-restful v2.10.0+incompatible // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/fullsailor/pkcs7 v0.0.0-20180422025557-ae226422660e
	github.com/ghodss/yaml v1.0.0
	github.com/go-ini/ini v1.25.4
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/jsonreference v0.19.3 // indirect
	github.com/go-openapi/runtime v0.19.6 // indirect
	github.com/gobuffalo/flect v0.1.6 // indirect
	github.com/gogo/protobuf v1.3.0
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/pprof v0.0.0-20190908185732-236ed259b199 // indirect
	github.com/gophercloud/gophercloud v0.4.0
	github.com/gorilla/mux v1.7.0
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.11.2 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/hashicorp/hcl v1.0.0
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/jacksontj/memberlistmesh v0.0.0-20190905163944-93462b9d2bb7
	github.com/jpillora/backoff v0.0.0-20170918002102-8eab2debe79d
	github.com/jteeuwen/go-bindata v0.0.0-20151023091102-a0ff2567cfb7
	github.com/jung-kurt/gofpdf v1.12.4 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pty v1.1.8 // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mailru/easyjson v0.7.0 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/miekg/coredns v0.0.0-20161111164017-20e25559d5ea
	github.com/miekg/dns v1.0.14
	github.com/mitchellh/mapstructure v1.1.2
	github.com/munnerz/goautoneg v0.0.0-20190414153302-2ae31c8b6b30 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/pborman/uuid v1.2.0
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/phpdave11/gofpdi v1.0.8 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pkg/sftp v0.0.0-20160930220758-4d0e916071f6
	github.com/pquerna/cachecontrol v0.0.0-20180517163645-1555304b9b35 // indirect
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/prometheus/common v0.7.0 // indirect
	github.com/prometheus/procfs v0.0.5 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20190728182440-6a916e37a237 // indirect
	github.com/rogpeppe/fastuuid v1.2.0 // indirect
	github.com/rogpeppe/go-charset v0.0.0-20190617161244-0dc95cdf6f31 // indirect
	github.com/rogpeppe/go-internal v1.4.0 // indirect
	github.com/russross/blackfriday v2.0.0+incompatible // indirect
	github.com/sergi/go-diff v0.0.0-20161102184045-552b4e9bbdca
	github.com/spf13/cobra v0.0.5
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.4.0
	github.com/spotinst/spotinst-sdk-go v0.0.0-20190505130751-eb52d7ac273c
	github.com/stretchr/testify v1.4.0
	github.com/ugorji/go v1.1.7 // indirect
	github.com/urfave/cli v1.20.0
	github.com/vmware/govmomi v0.20.1
	github.com/weaveworks/mesh v0.0.0-20170419100114-1f158d31de55
	go.etcd.io/bbolt v1.3.3 // indirect
	go.opencensus.io v0.22.1 // indirect
	go.uber.org/multierr v1.2.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190927123631-a832865fa7ad
	golang.org/x/exp v0.0.0-20190927203820-447a159532ef // indirect
	golang.org/x/image v0.0.0-20190910094157-69e4b8554b2a // indirect
	golang.org/x/mobile v0.0.0-20190923204409-d3ece3b6da5f // indirect
	golang.org/x/net v0.0.0-20190926025831-c00fd9afed17
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20190927073244-c990c680b611 // indirect
	golang.org/x/time v0.0.0-20190921001708-c4c64cad1fd0 // indirect
	golang.org/x/tools v0.0.0-20190929041059-e7abfedfabcf // indirect
	gonum.org/v1/gonum v0.0.0-20190926113837-94b2bbd8ac13 // indirect
	gonum.org/v1/netlib v0.0.0-20190926062253-2d6e29b73a19 // indirect
	gonum.org/v1/plot v0.0.0-20190615073203-9aa86143727f // indirect
	google.golang.org/api v0.10.0
	google.golang.org/appengine v1.6.4 // indirect
	google.golang.org/genproto v0.0.0-20190927181202-20e1ac93f88c // indirect
	google.golang.org/grpc v1.24.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/gcfg.v1 v1.2.0
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1 // indirect
	gopkg.in/yaml.v2 v2.2.2
	gopkg.in/yaml.v3 v3.0.0-20190924164351-c8b7dadae555 // indirect
	honnef.co/go/tools v0.0.1-2019.2.3
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/cli-runtime v0.0.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/gengo v0.0.0-20190907103519-ebc107f98eab // indirect
	k8s.io/helm v2.9.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d // indirect
	k8s.io/kubernetes v1.15.3
	k8s.io/legacy-cloud-providers v0.0.0
	k8s.io/utils v0.0.0-20190923111123-69764acb6e8e
	modernc.org/strutil v1.1.0 // indirect
	sigs.k8s.io/controller-runtime v0.2.2
	sigs.k8s.io/controller-tools v0.2.2-0.20190919191502-76a25b63325a
	sigs.k8s.io/structured-merge-diff v0.0.0-20190925174805-b04768683e36 // indirect
)
