##`KubeDNSConfig` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `KubeDNSConfig`



KubeDNSConfig defines the kube dns configuration

<aside class="notice">
Appears In:

<ul> 
<li><a href="#clusterspec-v1alpha2-kops">ClusterSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`cacheMaxConcurrent`<br /> *integer*    | CacheMaxConcurrent is the maximum number of concurrent queries for dnsmasq
`cacheMaxSize`<br /> *integer*    | CacheMaxSize is the maximum entries to keep in dnsmaq
`domain`<br /> *string*    | Domain is the dns domain
`image`<br /> *string*    | Image is the name of the docker image to run - @deprecated as this is now in the addon
`provider`<br /> *string*    | Provider indicates whether CoreDNS or kube-dns will be the default service discovery.
`replicas`<br /> *integer*    | Replicas is the number of pod replicas - @deprecated as this is now in the addon, and controlled by autoscaler
`serverIP`<br /> *string*    | ServerIP is the server ip
`stubDomains`<br /> *object*    | StubDomains redirects a domains to another DNS service
`upstreamNameservers`<br /> *string array*    | UpstreamNameservers sets the upstream nameservers for queries not on the cluster domain

