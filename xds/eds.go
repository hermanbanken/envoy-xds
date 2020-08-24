package main

import (
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/golang/protobuf/ptypes"
	_struct "github.com/golang/protobuf/ptypes/struct"
)

// EnvoyService is the resource type that is discovered by EDS
type EnvoyService struct {
	name      string
	port      uint32
	endpoints []EnvoyServiceEndpoint
}

// EnvoyServiceEndpoint is 1 single server & its metadata
type EnvoyServiceEndpoint struct {
	Address  string
	Metadata map[string]string
}

// MakeEndpointsForService converts from our convenience format to a ClusterLoadAssignment
func MakeEndpointsForService(service *EnvoyService) *endpoint.ClusterLoadAssignment {
	cla := &endpoint.ClusterLoadAssignment{
		ClusterName: service.name,
		Endpoints:   []*endpoint.LocalityLbEndpoints{},
	}

	for _, serviceEndpoint := range service.endpoints {
		cla.Endpoints = append(cla.Endpoints,
			&endpoint.LocalityLbEndpoints{
				LbEndpoints: []*endpoint.LbEndpoint{{
					// Point to a specified IP address
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: &core.Address{
								Address: &core.Address_SocketAddress{
									SocketAddress: &core.SocketAddress{
										Protocol: core.SocketAddress_TCP,
										Address:  serviceEndpoint.Address,
										PortSpecifier: &core.SocketAddress_PortValue{
											PortValue: service.port,
										},
									},
								},
							},
						},
					},
					// Allow filtering via metadata
					Metadata: &core.Metadata{
						FilterMetadata: map[string]*_struct.Struct{
							"envoy.lb": mapToStruct(serviceEndpoint.Metadata),
						},
					},
				}},
			},
		)
	}
	return cla
}

// MakeClusterForService returns an EDS cluster, filled in via endpoint resources
func MakeClusterForService(service *EnvoyService) *cluster.Cluster {
	return &cluster.Cluster{
		Name:                 service.name,
		ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_EDS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		EdsClusterConfig: &cluster.Cluster_EdsClusterConfig{
			EdsConfig: makeConfigSource(),
		},
		// LbSubsetConfig is how we can route traffic to specific backends
		// see https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/subsets
		LbSubsetConfig: &cluster.Cluster_LbSubsetConfig{
			FallbackPolicy: cluster.Cluster_LbSubsetConfig_ANY_ENDPOINT,
			DefaultSubset:  mapToStruct(map[string]string{"slice": "default"}),
			SubsetSelectors: []*cluster.Cluster_LbSubsetConfig_LbSubsetSelector{
				{Keys: []string{"slice"}, FallbackPolicy: cluster.Cluster_LbSubsetConfig_LbSubsetSelector_NO_FALLBACK},
			},
		},
	}
}

// Convert to annoying protobuf nested types...
func mapToStruct(m map[string]string) *_struct.Struct {
	fields := make(map[string]*_struct.Value, len(m))
	for key, value := range m {
		fields[key] = &_struct.Value{Kind: &_struct.Value_StringValue{StringValue: value}}
	}
	return &_struct.Struct{Fields: fields}
}
