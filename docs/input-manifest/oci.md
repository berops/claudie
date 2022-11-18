# OCI

In Claudie, OCI cloud provider requires you to input few variables which points to an account in your tenancy. To access the account, you need to provider tenancy OCID, user OCID, with user's API private key and fingerprint. Finally, you need to provide compartment OCID, which points to the compartment where resources for your infrastructure will be created.

However, in order to create those resources, user needs to be in a group with the sufficient IAM policies.

## IAM policies required by Claudie:
```
"Allow group <GROUP_NAME> to manage instance-family in compartment <COMPARTMENT_NAME>"
"Allow group <GROUP_NAME> to manage volume-family in compartment <COMPARTMENT_NAME>"
"Allow group <GROUP_NAME> to manage virtual-network-family in tenancy"
```