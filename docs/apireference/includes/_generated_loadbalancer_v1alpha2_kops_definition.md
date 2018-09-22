##`LoadBalancer` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `LoadBalancer`



LoadBalancer defines a load balancer

<aside class="notice">
Appears In:

<ul> 
<li><a href="#instancegroupspec-v1alpha2-kops">InstanceGroupSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`loadBalancerName`<br /> *string*    | LoadBalancerName to associate with this instance group (AWS ELB)
`targetGroupArn`<br /> *string*    | TargetGroupARN to associate with this instance group (AWS ALB/NLB)

