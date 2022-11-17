# OCI

In Claudie, OCI cloud provider requires you to input few variables which points to an account to your tenancy. To access that account, you need to provide private key and its fingerprint. Finally, you need to provide compartment OCID, which points to the compartment where resources for your infrastructure will be created.

However, in order to create those resources, user needs to be in a group with the sufficient identity policies.

## Policies required by Claudie:
```
"Allow group <GROUP_NAME> to manage instance-family in compartment <COMPARTMENT_NAME>"
"Allow group <GROUP_NAME> to manage volume-family in compartment <COMPARTMENT_NAME>"
"Allow group <GROUP_NAME> to manage virtual-network-family in tenancy"
```