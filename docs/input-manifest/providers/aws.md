# AWS

In Claudie, the AWS cloud provider requires you to input the credentials as an `access_key` and a `secret_key` which will be linked to the IAM user in your account. It is important, that said IAM user will have sufficient policies attached. This will assure that Claudie will be able to create all resources for your infrastructure.

## DNS requirements

If your AWS provider will be used for DNS, you need to manually
- [set up dns zone](https://aws.amazon.com/route53/)
- [update domain name server](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/GetInfoAboutHostedZone.html)

since Claudie does not support their dynamic creation.

## IAM policies required by Claudie:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:*"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "route53:*"
      ],
      "Resource": "*"
    }
  ]
}
```