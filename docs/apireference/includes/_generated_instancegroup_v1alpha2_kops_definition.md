##`InstanceGroup` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `InstanceGroup`



InstanceGroup represents a group of instances (either nodes or masters) with the same configuration

<aside class="notice">
Appears In:

<ul> 
<li><a href="#instancegrouplist-v1alpha2-kops">InstanceGroupList kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`apiVersion`<br /> *string*    | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources
`kind`<br /> *string*    | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds
`metadata`<br /> *ObjectMeta*    | 
`spec`<br /> *[InstanceGroupSpec](#instancegroupspec-v1alpha2-kops)*    | 

