# AWS

In Claudie, the AWS cloud provider requires you to input the credentials as an `access_key` and a `secret_key` which will be linked to the IAM user in your account. It is important, that said IAM user will have sufficient policies attached. This will assure that Claudie will be able to create all resources for your infrastructure.

## The policies required by Claudie:

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
    }
  ]
}
```