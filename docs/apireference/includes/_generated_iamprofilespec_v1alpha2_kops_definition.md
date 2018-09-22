##`IAMProfileSpec` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `IAMProfileSpec`



IAMProfileSpec is the AWS IAM Profile to attach to instances in this instance group. Specify the ARN for the IAM instance profile (AWS only).

<aside class="notice">
Appears In:

<ul> 
<li><a href="#instancegroupspec-v1alpha2-kops">InstanceGroupSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`profile`<br /> *string*    | Profile of the cloud group iam profile. In aws this is the arn for the iam instance profile

