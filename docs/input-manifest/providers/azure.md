# Azure

In Claudie, Azure cloud provider requires you to input a few variables in order to function properly. These variables are

- `subscription_id` which is the ID to your subscription. Bear in mind that all resources you define needs to be supported by that subscription and will be charged there.

- `tenant_id` which is the ID of your tenant in the active directory.

- `client_id` which is the ID for your service principal, under the tenancy you specified.

- `client_secret` which is the secret, for specified service principal.

Furthermore, service principal has to have a certain role assigned to it. For VM and VPC management it is `Virtual Machine Contributor` and `Network Contributor` respectively; and for resource group creation and deletion,the permission are

```tf
permissions {
    actions = [ 
      "Microsoft.Resources/subscriptions/resourceGroups/write",
      "Microsoft.Resources/subscriptions/resourceGroups/delete",
      ]
  }
```

## DNS requirements

If your Azure provider will be used for DNS, you need to manually

- [set up dns zone](https://learn.microsoft.com/en-us/azure/dns/dns-getstarted-portal)
- [update domain name server](https://learn.microsoft.com/en-us/azure/dns/dns-getstarted-portal#test-the-name-resolution)

since Claudie does not support their dynamic creation.