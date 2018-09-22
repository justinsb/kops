##`KubeAPIServerConfig` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `KubeAPIServerConfig`



KubeAPIServerConfig defines the configuration for the kube api

<aside class="notice">
Appears In:

<ul> 
<li><a href="#clusterspec-v1alpha2-kops">ClusterSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`address`<br /> *string*    | Address is the binding address for the kube api: Deprecated - use insecure-bind-address and bind-address
`admissionControl`<br /> *string array*    | Deprecated: AdmissionControl is a list of admission controllers to use
`allowPrivileged`<br /> *boolean*    | AllowPrivileged indicates if we can run privileged containers
`anonymousAuth`<br /> *boolean*    | AnonymousAuth indicates if anonymous authentication is permitted
`apiServerCount`<br /> *integer*    | APIServerCount is the number of api servers
`auditLogFormat`<br /> *string*    | AuditLogFormat flag specifies the format type for audit log files.
`auditLogMaxAge`<br /> *integer*    | The maximum number of days to retain old audit log files based on the timestamp encoded in their filename.
`auditLogMaxBackups`<br /> *integer*    | The maximum number of old audit log files to retain.
`auditLogMaxSize`<br /> *integer*    | The maximum size in megabytes of the audit log file before it gets rotated. Defaults to 100MB.
`auditLogPath`<br /> *string*    | If set, all requests coming to the apiserver will be logged to this file.
`auditPolicyFile`<br /> *string*    | AuditPolicyFile is the full path to a advanced audit configuration file a.g. /srv/kubernetes/audit.conf
`authenticationTokenWebhookCacheTtl`<br /> *Duration*    | The duration to cache responses from the webhook token authenticator. Default is 2m. (default 2m0s)
`authenticationTokenWebhookConfigFile`<br /> *string*    | File with webhook configuration for token authentication in kubeconfig format. The API server will query the remote service to determine authentication for bearer tokens.
`authorizationMode`<br /> *string*    | AuthorizationMode is the authorization mode the kubeapi is running in
`authorizationRbacSuperUser`<br /> *string*    | AuthorizationRBACSuperUser is the name of the superuser for default rbac
`basicAuthFile`<br /> *string*    | 
`bindAddress`<br /> *string*    | BindAddress is the binding address for the secure kubernetes API
`clientCAFile`<br /> *string*    | 
`cloudProvider`<br /> *string*    | CloudProvider is the name of the cloudProvider we are using, aws, gce etcd
`disableAdmissionPlugins`<br /> *string array*    | DisableAdmissionPlugins is a list of disabled admission plugins
`enableAdmissionPlugins`<br /> *string array*    | EnableAdmissionPlugins is a list of enabled admission plugins
`enableAggregatorRouting`<br /> *boolean*    | EnableAggregatorRouting enables aggregator routing requests to endpoints IP rather than cluster IP
`enableBootstrapTokenAuth`<br /> *boolean*    | EnableBootstrapAuthToken enables &#39;bootstrap.kubernetes.io/token&#39; in the &#39;kube-system&#39; namespace to be used for TLS bootstrapping authentication
`etcdCaFile`<br /> *string*    | EtcdCAFile is the path to a ca certificate
`etcdCertFile`<br /> *string*    | EtcdCertFile is the path to a certificate
`etcdKeyFile`<br /> *string*    | EtcdKeyFile is the path to a private key
`etcdQuorumRead`<br /> *boolean*    | EtcdQuorumRead configures the etcd-quorum-read flag, which forces consistent reads from etcd
`etcdServers`<br /> *string array*    | EtcdServers is a list of the etcd service to connect
`etcdServersOverrides`<br /> *string array*    | EtcdServersOverrides is per-resource etcd servers overrides, comma separated. The individual override format: group/resource#servers, where servers are http://ip:port, semicolon separated
`experimentalEncryptionProviderConfig`<br /> *string*    | ExperimentalEncryptionProviderConfig enables encryption at rest for secrets.
`featureGates`<br /> *object*    | FeatureGates is set of key=value pairs that describe feature gates for alpha/experimental features.
`image`<br /> *string*    | Image is the docker container used
`insecureBindAddress`<br /> *string*    | InsecureBindAddress is the binding address for the InsecurePort for the insecure kubernetes API
`insecurePort`<br /> *integer*    | InsecurePort is the port the insecure api runs
`kubeletClientCertificate`<br /> *string*    | KubeletClientCertificate is the path of a certificate for secure communication between api and kubelet
`kubeletClientKey`<br /> *string*    | KubeletClientKey is the path of a private to secure communication between api and kubelet
`kubeletPreferredAddressTypes`<br /> *string array*    | KubeletPreferredAddressTypes is a list of the preferred NodeAddressTypes to use for kubelet connections
`logLevel`<br /> *integer*    | LogLevel is the logging level of the api
`maxRequestsInflight`<br /> *integer*    | MaxRequestsInflight The maximum number of non-mutating requests in flight at a given time.
`minRequestTimeout`<br /> *integer*    | MinRequestTimeout configures the minimum number of seconds a handler must keep a request open before timing it out. Currently only honored by the watch request handler
`oidcCAFile`<br /> *string*    | OIDCCAFile if set, the OpenID server&#39;s certificate will be verified by one of the authorities in the oidc-ca-file
`oidcClientID`<br /> *string*    | OIDCClientID is the client ID for the OpenID Connect client, must be set if oidc-issuer-url is set.
`oidcGroupsClaim`<br /> *string*    | OIDCGroupsClaim if provided, the name of a custom OpenID Connect claim for specifying user groups. The claim value is expected to be a string or array of strings.
`oidcGroupsPrefix`<br /> *string*    | OIDCGroupsPrefix is the prefix prepended to group claims to prevent clashes with existing names (such as &#39;system:&#39; groups)
`oidcIssuerURL`<br /> *string*    | OIDCIssuerURL is the URL of the OpenID issuer, only HTTPS scheme will be accepted. If set, it will be used to verify the OIDC JSON Web Token (JWT).
`oidcUsernameClaim`<br /> *string*    | OIDCUsernameClaim is the OpenID claim to use as the user name. Note that claims other than the default (&#39;sub&#39;) is not guaranteed to be unique and immutable.
`oidcUsernamePrefix`<br /> *string*    | OIDCUsernamePrefix is the prefix prepended to username claims to prevent clashes with existing names (such as &#39;system:&#39; users).
`proxyClientCertFile`<br /> *string*    | The apiserver&#39;s client certificate used for outbound requests.
`proxyClientKeyFile`<br /> *string*    | The apiserver&#39;s client key used for outbound requests.
`requestheaderAllowedNames`<br /> *string array*    | List of client certificate common names to allow to provide usernames in headers specified by --requestheader-username-headers. If empty, any client certificate validated by the authorities in --requestheader-client-ca-file is allowed.
`requestheaderClientCAFile`<br /> *string*    | Root certificate bundle to use to verify client certificates on incoming requests before trusting usernames in headers specified by --requestheader-username-headers
`requestheaderExtraHeaderPrefixes`<br /> *string array*    | List of request header prefixes to inspect. X-Remote-Extra- is suggested.
`requestheaderGroupHeaders`<br /> *string array*    | List of request headers to inspect for groups. X-Remote-Group is suggested.
`requestheaderUsernameHeaders`<br /> *string array*    | List of request headers to inspect for usernames. X-Remote-User is common.
`runtimeConfig`<br /> *object*    | RuntimeConfig is a series of keys/values are parsed into the `--runtime-config` parameters
`securePort`<br /> *integer*    | SecurePort is the port the kube runs on
`serviceClusterIPRange`<br /> *string*    | ServiceClusterIPRange is the service address range
`serviceNodePortRange`<br /> *string*    | Passed as --service-node-port-range to kube-apiserver. Expects &#39;startPort-endPort&#39; format. Eg. 30000-33000
`storageBackend`<br /> *string*    | StorageBackend is the backend storage
`tlsCertFile`<br /> *string*    | 
`tlsPrivateKeyFile`<br /> *string*    | 
`tokenAuthFile`<br /> *string*    | 

