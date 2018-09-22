##`KeysetItem` [`kops`/`v1alpha2`]

Group        | Version     | Kind
------------ | ---------- | -----------
`kops` | `v1alpha2` | `KeysetItem`



KeysetItem is an item (keypair or other secret material) in a Keyset

<aside class="notice">
Appears In:

<ul> 
<li><a href="#keysetspec-v1alpha2-kops">KeysetSpec kops/v1alpha2</a></li>
</ul></aside>

Field        | Description
------------ | -----------
`id`<br /> *string*    | Id is the unique identifier for this key in the keyset
`privateMaterial`<br /> *string*    | PrivateMaterial holds secret material (e.g. a private key, or symmetric token)
`publicMaterial`<br /> *string*    | PublicMaterial holds non-secret material (e.g. a certificate)

