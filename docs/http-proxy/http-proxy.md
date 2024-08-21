# Usage of HTTP proxy

In this section, we'll describe the default HTTP proxy setup and its the further customization.

## Default setup

By default `HTTP_PROXY_MODE` is set to `default` [see](https://docs.claudie.io/latest/getting-started/detailed-guide/#claudie-customization), thus Claudie utilizes the HTTP proxy in building the K8s cluster only when there is at least one node from the Hetzner cloud provider. This means, that if you have a cluster with one master node in Azure and one worker node in AWS Claudie won't use the HTTP proxy to build the K8s cluster. However, if you add another worker node from Hetzner the whole process of building the K8s cluster will utilize the HTTP proxy. 

This approach was implemented to address the following issues:

- [https://github.com/berops/claudie/issues/783](https://github.com/berops/claudie/issues/783)
- [https://github.com/berops/claudie/issues/1272](https://github.com/berops/claudie/issues/1272)

## Further customization

In case you don't want to utilize the HTTP proxy at all (even when there are nodes in the K8s cluster from the Hetzner cloud provider) you can turn off the HTTP proxy by setting the `HTTP_PROXY_MODE` to `off` ([see](https://docs.claudie.io/latest/getting-started/detailed-guide/#claudie-customization)). On the other hand, if you wish to use the HTTP proxy whenever building a K8s cluster (even when there aren't any nodes in the K8s cluster from the Hetzner cloud provider) you can set the `HTTP_PROXY_MODE` to `on` ([see](https://docs.claudie.io/latest/getting-started/detailed-guide/#claudie-customization)).

If you want to utilize your own HTTP proxy you can set its URL in `HTTP_PROXY_URL` ([see](https://docs.claudie.io/latest/getting-started/detailed-guide/#claudie-customization)). By default, this value is set to `http://proxy.claudie.io:8880`. In case your HTTP proxy runs on `myproxy.com` and is exposed on port `3128` the `HTTP_PROXY_URL` has to be set to `http://myproxy.com:3128`. This means you always have to specify the whole URL with the protocol (HTTP), domain name, and port. 
