##`NodeAuthorizerSpec` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `NodeAuthorizerSpec`



NodeAuthorizerSpec defines the configuration for a node authorizer

<aside class="notice">
Appears In:

<ul> 
<li><a href="#nodeauthorizationspec-v1alpha2-kops">NodeAuthorizationSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`authorizer`<br /> *string*    | Authorizer is the authorizer to use
`features`<br /> *string array*    | Features is a series of authorizer features to enable or disable
`image`<br /> *string*    | Image is the location of container
`nodeURL`<br /> *string*    | NodeURL is the node authorization service url
`port`<br /> *integer*    | Port is the port the service is running on the master
`timeout`<br /> *Duration*    | Timeout the max time for authorization request
`tokenTTL`<br /> *Duration*    | TokenTTL is the max ttl for an issued token

