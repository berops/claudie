# Deploying Node-Local-DNS

Claudie doesn't deploy `node-local-dns` by default. In this section we'll walk through an example
of how to deploy `node-local-dns` for a claudie created cluster.

### 1. Download `nodelocaldns.yaml`

Based on the kubernetes version you are using in your cluster download the `nodelocaldns.yaml`
from the kubernetes [repository](https://github.com/kubernetes/kubernetes/blob/master/cluster/addons/dns/nodelocaldns/nodelocaldns.yaml)

Make sure to download the YAML for the right kubernetes version, e.g. for kubernetes version 1.27 you would use:

```bash
wget https://raw.githubusercontent.com/kubernetes/kubernetes/release-1.27/cluster/addons/dns/nodelocaldns/nodelocaldns.yaml
```

### 2. Modify downloaded `nodelocaldns.yaml`

We'll need to replace the references to `__PILLAR__DNS__DOMAIN__` and some of the references to `__PILLAR__LOCAL__DNS__`

To replace `__PILLAR__DNS__DOMAIN__` execute:

```bash
sed -i "s/__PILLAR__DNS__DOMAIN__/cluster.local/g" nodelocaldns.yaml
```

To replace `__PILLAR__LOCAL__DNS__` find the references and change it to [169.254.20.10](https://github.com/kubermatic/kubeone/blob/515d7a3b1dbf42a4f04fae6dccdcb86eaa77e238/pkg/templates/resources/resources.go#L85) as shown below:

```dif
    ...
      containers:
      - name: node-cache
        image: registry.k8s.io/dns/k8s-dns-node-cache:1.22.20
        resources:
          requests:
            cpu: 25m
            memory: 5Mi
-       args: [ "-localip", "__PILLAR__LOCAL__DNS__,__PILLAR__DNS__SERVER__", "-conf", "/etc/Corefile", "-upstreamsvc", "kube-dns-upstream" ]
+       args: [ "-localip", "169.254.20.10", "-conf", "/etc/Corefile", "-upstreamsvc", "kube-dns-upstream" ]
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        - containerPort: 9253
          name: metrics
          protocol: TCP
        livenessProbe:
          httpGet:
-           host: __PILLAR__LOCAL__DNS__
+           host: 169.254.20.10
            path: /health
            port: 8080
          initialDelaySeconds: 60
          timeoutSeconds: 5
    ...
```

### 3. Apply the modified manifest.

`kubectl apply -f ./nodelocaldns.yaml`