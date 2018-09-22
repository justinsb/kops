##`CiliumNetworkingSpec` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `CiliumNetworkingSpec`



CiliumNetworkingSpec declares that we want Cilium networking

<aside class="notice">
Appears In:

<ul> 
<li><a href="#networkingspec-v1alpha2-kops">NetworkingSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`accessLog`<br /> *string*    | 
`agentLabels`<br /> *string array*    | 
`allowLocalhost`<br /> *string*    | 
`autoIpv6NodeRoutes`<br /> *boolean*    | 
`bpfRoot`<br /> *string*    | 
`containerRuntime`<br /> *string array*    | 
`containerRuntimeEndpoint`<br /> *object*    | 
`debug`<br /> *boolean*    | 
`debugVerbose`<br /> *string array*    | 
`device`<br /> *string*    | 
`disableConntrack`<br /> *boolean*    | 
`disableIpv4`<br /> *boolean*    | 
`disableK8sServices`<br /> *boolean*    | 
`disableMasquerade`<br /> *boolean*    | 
`enablePolicy`<br /> *string*    | 
`enableTracing`<br /> *boolean*    | 
`envoyLog`<br /> *string*    | 
`ipv4ClusterCidrMaskSize`<br /> *integer*    | 
`ipv4Node`<br /> *string*    | 
`ipv4Range`<br /> *string*    | 
`ipv4ServiceRange`<br /> *string*    | 
`ipv6ClusterAllocCidr`<br /> *string*    | 
`ipv6Node`<br /> *string*    | 
`ipv6Range`<br /> *string*    | 
`ipv6ServiceRange`<br /> *string*    | 
`k8sApiServer`<br /> *string*    | 
`k8sKubeconfigPath`<br /> *string*    | 
`keepBpfTemplates`<br /> *boolean*    | 
`keepConfig`<br /> *boolean*    | 
`labelPrefixFile`<br /> *string*    | 
`labels`<br /> *string array*    | 
`lb`<br /> *string*    | 
`libDir`<br /> *string*    | 
`logDriver`<br /> *string array*    | 
`logOpt`<br /> *object*    | 
`logstash`<br /> *boolean*    | 
`logstashAgent`<br /> *string*    | 
`logstashProbeTimer`<br /> *integer*    | 
`nat46Range`<br /> *string*    | 
`pprof`<br /> *boolean*    | 
`prefilterDevice`<br /> *string*    | 
`prometheusServeAddr`<br /> *string*    | 
`restore`<br /> *boolean*    | 
`singleClusterRoute`<br /> *boolean*    | 
`socketPath`<br /> *string*    | 
`stateDir`<br /> *string*    | 
`tracePayloadlen`<br /> *integer*    | 
`tunnel`<br /> *string*    | 
`version`<br /> *string*    | 

