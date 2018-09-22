##`KubeProxyConfig` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `KubeProxyConfig`



KubeProxyConfig defines the configuration for a proxy

<aside class="notice">
Appears In:

<ul> 
<li><a href="#clusterspec-v1alpha2-kops">ClusterSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`bindAddress`<br /> *string*    | BindAddress is IP address for the proxy server to serve on
`clusterCIDR`<br /> *string*    | ClusterCIDR is the CIDR range of the pods in the cluster
`conntrackMaxPerCore`<br /> *integer*    | Maximum number of NAT connections to track per CPU core (default: 131072)
`conntrackMin`<br /> *integer*    | Minimum number of conntrack entries to allocate, regardless of conntrack-max-per-core
`cpuLimit`<br /> *string*    | CPULimit, cpu limit compute resource for kube proxy e.g. &#34;30m&#34;
`cpuRequest`<br /> *string*    | CPURequest, cpu request compute resource for kube proxy e.g. &#34;20m&#34;
`enabled`<br /> *boolean*    | Enabled allows enabling or disabling kube-proxy
`featureGates`<br /> *object*    | FeatureGates is a series of key pairs used to switch on features for the proxy
`hostnameOverride`<br /> *string*    | HostnameOverride, if non-empty, will be used as the identity instead of the actual hostname.
`image`<br /> *string*    | 
`logLevel`<br /> *integer*    | LogLevel is the logging level of the proxy
`master`<br /> *string*    | Master is the address of the Kubernetes API server (overrides any value in kubeconfig)
`memoryLimit`<br /> *string*    | MemoryLimit, memory limit compute resource for kube proxy e.g. &#34;30Mi&#34;
`memoryRequest`<br /> *string*    | MemoryRequest, memory request compute resource for kube proxy e.g. &#34;30Mi&#34;
`proxyMode`<br /> *string*    | Which proxy mode to use: (userspace, iptables, ipvs)

