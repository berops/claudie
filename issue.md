<!--
Note: Please search to see if an issue already exists for the feature you request.
-->

### Motivation:
<!-- A concise description of what you're experiencing. -->
Currently, we are treating every provider as if they need the same specification. However, this is not always the case. I briefly discussed it with the @jaskeerat789 and we agreed that best approach would be to define every supported provider as their own thing, i.e.

Current
```yaml
providers:
  - name: hetzner
    credentials: abcd1234
  - name: gcp
    credentials: sak.json
```
New
```yaml
providers:
  - hetzner: 
      name: hetzner-1
      credentials: abcd1234
  - gcp:
      name: gcp-1
      credenials: sak.json
      project: project-1
```
Futhermore, we could also define stuff like `region` and `zone` which are currently specified for every nodepool, which leads to a lot of repetition. Instead of specifying it under nodepools, we could just reference a provider name, i.e.

Current
```yaml
providers:
  - name: hetzner
    credentials: abcd1234
  - name: gcp
    credentials: sak.json
....
nodePools:
  dynamic:
    - name: gcp-control-1
      provider:
        gcp:
          region: europe-west1
          zone: europe-west1-c
    - name: gcp-control-2
      provider:
        gcp:
          region: europe-west1
          zone: europe-west1-a
```
New
```yaml
providers:
  gcp:
    name: gcp-1
    credentials: sak.json
    region: europe-west1
    zone: europe-west1-c
  gcp:
    credentials: sak.json
    region: europe-west1
    zone: europe-west1-a
.....
nodePools:
  dynamic:
    - name: gcp-control-1
      provider: gcp-1
      ......
    - name: gcp-control-2
      provider: gcp-2
      .....
```

### Description:
<!-- A concise description of what you expected to happen. -->

### Exit criteria:
- [ ] Task...
<!--
Example:
- [ ] Implement the feature
- [ ] Add feature to the platform
-->
