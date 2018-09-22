##`DockerConfig` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `DockerConfig`



DockerConfig is the configuration for docker

<aside class="notice">
Appears In:

<ul> 
<li><a href="#clusterspec-v1alpha2-kops">ClusterSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`authorizationPlugins`<br /> *string array*    | AuthorizationPlugins is a list of authorization plugins
`bridge`<br /> *string*    | Bridge is the network interface containers should bind onto
`bridgeIP`<br /> *string*    | BridgeIP is a specific IP address and netmask for the docker0 bridge, using standard CIDR notation
`dataRoot`<br /> *string*    | DataRoot is the root directory of persistent docker state (default &#34;/var/lib/docker&#34;)
`defaultUlimit`<br /> *string array*    | DefaultUlimit is the ulimits for containers
`execRoot`<br /> *string*    | ExecRoot is the root directory for execution state files (default &#34;/var/run/docker&#34;)
`hosts`<br /> *string array*    | Hosts enables you to configure the endpoints the docker daemon listens on i.e tcp://0.0.0.0.2375 or unix:///var/run/docker.sock etc
`insecureRegistry`<br /> *string*    | InsecureRegistry enable insecure registry communication @question according to dockers this a list??
`ipMasq`<br /> *boolean*    | IPMasq enables ip masquerading for containers
`ipTables`<br /> *boolean*    | IPtables enables addition of iptables rules
`liveRestore`<br /> *boolean*    | LiveRestore enables live restore of docker when containers are still running
`logDriver`<br /> *string*    | LogDriver is the default driver for container logs (default &#34;json-file&#34;)
`logLevel`<br /> *string*    | LogLevel is the logging level (&#34;debug&#34;, &#34;info&#34;, &#34;warn&#34;, &#34;error&#34;, &#34;fatal&#34;) (default &#34;info&#34;)
`logOpt`<br /> *string array*    | Logopt is a series of options given to the log driver options for containers
`mtu`<br /> *integer*    | MTU is the containers network MTU
`registryMirrors`<br /> *string array*    | RegistryMirrors is a referred list of docker registry mirror
`storage`<br /> *string*    | Storage is the docker storage driver to use
`storageOpts`<br /> *string array*    | StorageOpts is a series of options passed to the storage driver
`userNamespaceRemap`<br /> *string*    | UserNamespaceRemap sets the user namespace remapping option for the docker daemon
`version`<br /> *string*    | Version is consumed by the nodeup and used to pick the docker version

