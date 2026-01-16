# Roadmap for Claudie

Planned features (sorted by priority from highest):
- [ ] Reconciliation loop
- [ ] Event-based messaging for better parallelism
- [ ] OpenTofu provider caching
- [ ] Allow different OpenTofu template versions for a single provider

Unplanned features (wishlist; talk to us for prioritization):
- [ ] CLI read-only interface
- [ ] Override for all manifest defaults
- [ ] Service type: loadbalancer
- [ ] Support for Spot & preemptible instances
- [ ] Roadwarrior/Edge mode (on-prem node behind a NAT)

v0.9.14:
- [x] Support for OpenStack provider
- [x] Support for OVH provider
v0.9.0:
- [x] Override for all OpenTofu templates
v0.8.1:
- [x] Support for more cloud providers
    - [x] OCI
    - [x] AWS
    - [x] Azure
    - [x] Cloudflare
    - [x] GenesisCloud
- [x] Hybrid-cloud support (on-premises)
- [x] `arm64` support for the nodepools
- [x] App-level metrics
- [x] Autoscaler
