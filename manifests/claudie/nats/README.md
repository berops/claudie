# How to generate the deployment files

0. helm repo add nats https://nats-io.github.io/k8s/helm/charts/
1. The crds.yaml was taken from https://github.com/nats-io/k8s/blob/nats-2.12.3/helm/charts/nack/crds/crds.yml
2. helm template nats nats/nats \
    --set config.cluster.replicas=3
    --set config.jetstream.enabled=true \
    --set config.cluster.enabled=true \
    --set container.resources.requests.cpu=10m \
    --set container.resources.requests.memory=32Mi \
    --set container.resources.limits.cpu=100m \
    --set container.resources.limits.memory=128Mi \
    --set reloader.resources.requests.cpu=1m \
    --set reloader.resources.requests.memory=8Mi \
    --set reloader.resources.limits.cpu=10m \
    --set reloader.resources.limits.memory=16Mi \
    --set natsBox.enabled=false \
    --version 2.12.3 > ./nats/nats.yaml
3. helm template nack nats/nack \
    --set jetstream.nats.url=nats://nats.claudie.svc.cluster.local:4222 \
    --set resources.requests.cpu=5m \
    --set resources.requests.memory=16Mi \
    --set resources.limits.cpu=50m \
    --set resources.limits.memory=64Mi \
    --version 0.31.1 > ./nats/nack.yaml
4.  
Add
```
  labels:
    app.kubernetes.io/part-of: claudie
```
to All generated resources.

5. kustomize build ./nats | k apply -f -
