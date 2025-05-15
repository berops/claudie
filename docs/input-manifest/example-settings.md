Claudie allows to customize some aspects of the workflow when deploying kubernetes clusters.

To see where custom configurations can be applied, look for the `settingsRef` fields in the Input Manifest or read through the [api-reference](/input-manifest/api-reference/)

A Setting is a separate [Custom Resource Definition](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) which stores configurations that then replace the defaults used by Claudie when building clusters that reference these settings.

The following is an example replacing the configuration for envoy proxy which is deployed on the load balancers nodes.

```yaml
apiVersion: claudie.io/v1beta1
kind: Setting
metadata:
  name: custom-envoy
  namespace: claudie
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  envoy:
    lds: |
    resources:
      - "@type": type.googleapis.com/envoy.config.listener.v3.Listener
        name: "{{ $.Role.Name }}_listener"
        address:
          socket_address:
            protocol: "{{ $.Role.Protocol }}"
            address: 0.0.0.0
            port_value: {{ $.Role.Port }}
        listener_filters:
        {{- if eq $.Role.Protocol "udp" }}
          - name: envoy.filters.udp_listener.udp_proxy
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.udp.udp_proxy.v3.UdpProxyConfig
              stat_prefix: "{{ $.Role.Name }}"
              cluster: "{{ $.Role.Name }}"
              {{ if and $.Role.Settings.StickySessions (ne $.Role.TargetPort 6443) -}}
              hash_policies:
                - source_ip: true
              {{ end -}}
              access_log:
                - name: envoy.access_loggers.stdout
                  typed_config:
                    "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
        {{- end }}
        filter_chains:
        {{- if eq $.Role.Protocol "tcp" }}
          - filters:
              - name: envoy.filters.network.tcp_proxy
                typed_config:
                  "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
                  stat_prefix: "{{ $.Role.Name }}"
                  cluster: "{{ $.Role.Name }}"
                  max_connect_attempts: 3
                  {{ if and $.Role.Settings.StickySessions (ne $.Role.TargetPort 6443) -}}
                  hash_policy:
                    - source_ip: {}
                  {{ end -}}
                  access_log:
                    - name: envoy.access_loggers.stdout
                      typed_config:
                        "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
        {{- end }}
    cds: |
    resources:
      - "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
        name: "{{ .Role.Name }}"
        type: STATIC
        {{ if and .Role.Settings.StickySessions (ne .Role.TargetPort 6443) -}}
        lb_policy: RING_HASH
        {{ else -}}
        lb_policy: ROUND_ROBIN
        {{ end -}}
        connect_timeout: 10s
        {{ if and (and $.Role.Settings.ProxyProtocol (ne $.Role.TargetPort 6443)) (eq $.Role.Protocol "tcp") -}}
        transport_socket:
          name: envoy.transport_sockets.upstream_proxy_protocol
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.transport_sockets.proxy_protocol.v3.ProxyProtocolUpstreamTransport
            transport_socket:
              name: envoy.transport_sockets.raw_buffer
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.transport_sockets.raw_buffer.v3.RawBuffer
        {{ end -}}
        circuit_breakers:
          thresholds:
            max_connections: 65535
            max_pending_requests: 65535
            max_requests: 65535
        load_assignment:
          cluster_name: "{{ .Role.Name }}"
          endpoints:
            - lb_endpoints:
              {{- range $node := .TargetNodes }}
                - endpoint:
                    address:
                      socket_address:
                        address: {{ $node.Private }}
                        port_value: {{ $.Role.TargetPort }}
              {{- end }}
```

This setting can then be reference by a role definition inside the Input Manifest to take effect.

```diff
...
  loadBalancers:
    roles:
      - name: apiserver
        protocol: tcp
        port: 6443
        targetPort: 6443
        targetPools:
            - control-pools
        settings:
            proxyProtocol: true
            stickySessions: false
+        settingsRef:
+          name: custom-envoy
+          namespace: claudie
...
```
