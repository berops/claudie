## Testing Framework 

`testing-framework` is a custom service used to run tests by applying some pre-defined manifest and monitor the cluster creation process and their health after successful creation.

Please make sure you use the testing framework **only** for development purposes. \
You almost certainly don't want to deploy the testing framework as a regular user.

## Adding new testsets
To add another test-set create a directory in the `./test-sets` with the name of the next test scenario. In that directory create `.yaml` manifests for `inputmanifest` resource type - where the `.metadata.name` is the same as the test-set directory name.
For example, to create a new test set named `test-gcp-aws-1`, create a directory: `mkdir ./test-sets/test-gcp-aws-1`, and in that directory define a `yaml` manifest:
```
# 1.yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: test-gcp-aws-1
spec:

    ...
```
**Do not define a `.metadata.namespace` since each test run will have a different Namespace.**
