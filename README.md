# Envoy xDS demo
xDS is the control plane protocol of Envoy. An Envoy instance can be configured
to contact an xDS endpoint for additional config (see [envoy-data/bootstrap-xdsv3.yaml](./envoy-data/bootstrap-xdsv3.yml)).

Via xDS these 5 root level configuration types can be configured:
1. endpoints (implement only this for just an Endpoint Discovery Service (EDS))
2. clusters
3. routes
4. listeners
5. runtimes

This repository shows:
- how to run a custom xDS control-plane; uses [go-control-plane example](https://github.com/envoyproxy/go-control-plane/tree/master/internal/example/).
- how to run a EDS discovery service; uses [go-control-plane dyplomat example](https://github.com/envoyproxy/go-control-plane/tree/master/examples/dyplomat/).
- how to add metadata via web assembly.
- how to add metadata via a gRPC call.
- how to dynamically select cluster endpoints based on some kind of metadata.

# References
1. https://github.com/envoyproxy/go-control-plane/tree/master/internal/example/
2. https://github.com/envoyproxy/go-control-plane/tree/master/examples/dyplomat/
3. https://docs.solo.io/gloo/latest/guides/dev/writing-upstream-plugins/
