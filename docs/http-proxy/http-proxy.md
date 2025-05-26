# Usage of HTTP proxy

In this section, we'll describe the default HTTP proxy setup and its the further customization.

## Default setup

By default installation proxy mode is set to `default`, thus Claudie utilizes the HTTP proxy when building a K8s cluster with at least one node from the Hetzner cloud provider. This means, that if you have a cluster with one master node in Azure and one worker node in AWS Claudie won't use the HTTP proxy to build the K8s cluster. However, if you add another worker node from Hetzner the whole process of building the K8s cluster will utilize the HTTP proxy.

This approach was implemented to address the following issues:

- [https://github.com/berops/claudie/issues/783](https://github.com/berops/claudie/issues/783)
- [https://github.com/berops/claudie/issues/1272](https://github.com/berops/claudie/issues/1272)

## Further customization

In case you don't want to utilize the HTTP proxy at all (even when there are nodes in the K8s cluster from the Hetzner cloud provider) you can turn off the installation proxy by setting the proxy mode to `off` in the InputManifest (see the example below).

```
kubernetes:
    clusters:
      - name: proxy-example
        version: "1.30.0"
        network: 192.168.2.0/24
        installationProxy:
            mode: "off"
```

On the other hand, if you wish to use the HTTP proxy whenever building a K8s cluster (even when there aren't any nodes in the K8s cluster from the Hetzner cloud provider) you can set the proxy mode to `on` in the InputManifest (again, see the example below).

```
kubernetes:
    clusters:
      - name: proxy-example
        version: "1.30.0"
        network: 192.168.2.0/24
        installationProxy:
            mode: "on"
```

If you want to utilize your own HTTP proxy you can set its URL in `endpoint` (see the example below).

```
kubernetes:
    clusters:
      - name: proxy-example
        version: "1.30.0"
        network: 192.168.2.0/24
        installationProxy:
            mode: "on"
            endpoint: http://<my-endpoint-domain-name>:<my-endpoint-port>
```

By default, `endpoint` value is set to `http://proxy.claudie.io:8880`. In case your HTTP proxy runs on `myproxy.com` and is exposed on port `3128` the `endpoint` has to be set to `http://myproxy.com:3128`. This means you always have to specify the whole URL with the protocol (HTTP or HTTPS), domain name, and port.

 The Claudie proxy strictly limits the endpoints it allows. By default, it only allows endpoints for commonly used package and container registries, in order to prevent HTTP 403 errors when setting up a cluster with nodes that may have misused IP addresses assigned.
 This may not suit your needs if you use/need private repositories for your deployments, however. There is an additional field called `noProxy` that allows you to specify any endpoints that should not be routed through the proxy. The most common scenario would be downloading container images from private registries.

 The below example allows to bypass the Proxy for any endpoint ending with `.suse.com`.

```
kubernetes:
    clusters:
      - name: proxy-example
        version: "1.30.0"
        network: 192.168.2.0/24
        installationProxy:
            mode: "on"
            noProxy: ".suse.com"
```

!!! note "We gradually expand the default NoProxy list, if you think there is a repository or container registry that should be added you can always let us know"
