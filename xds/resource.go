// Copyright 2020 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.
package main

import (
	"fmt"

	"github.com/golang/protobuf/ptypes"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	header_to_metadata "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/header_to_metadata/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
)

func makeRoute(routeName string, clusterName string, hostRewriteLiteral string) *route.RouteConfiguration {
	return &route.RouteConfiguration{
		Name: routeName,
		VirtualHosts: []*route.VirtualHost{{
			Name:    "local_service",
			Domains: []string{"*"},
			Routes: []*route.Route{{
				Match: &route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{
						Prefix: "/",
					},
				},
				Action: &route.Route_Route{
					Route: &route.RouteAction{
						ClusterSpecifier: &route.RouteAction_Cluster{
							Cluster: clusterName,
						},
						HostRewriteSpecifier: &route.RouteAction_HostRewriteLiteral{
							HostRewriteLiteral: hostRewriteLiteral,
						},
					},
				},
			}},
		}},
	}
}

func makeHTTPListener(listenerName string, listenerPort uint32, route string) *listener.Listener {
	// HTTP header_to_metadata.Config must be converted to Any into HttpFilter_TypedConfig.TypedConfig
	typedConfig := &header_to_metadata.Config{
		// see https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/header_to_metadata_filter
		RequestRules: []*header_to_metadata.Config_Rule{{
			Header: "x-slice",
			OnHeaderPresent: &header_to_metadata.Config_KeyValuePair{
				Key:               "slice",
				MetadataNamespace: "envoy.lb",
				Type:              header_to_metadata.Config_STRING,
			},
			Remove: false,
		}},
	}
	if err := typedConfig.Validate(); err != nil {
		panic(err)
	}
	config, err := ptypes.MarshalAny(typedConfig)
	if err != nil {
		panic(err)
	}

	// HTTP filter configuration
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource:    makeConfigSource(),
				RouteConfigName: route,
			},
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name:       "envoy.filters.http.header_to_metadata",
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: config},
		}, {
			Name: wellknown.Router,
		}},
	}
	pbst, err := ptypes.MarshalAny(manager)
	if err != nil {
		panic(err)
	}

	return &listener.Listener{
		Name: listenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: listenerPort,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}
}

func makeConfigSource() *core.ConfigSource {
	source := &core.ConfigSource{}
	source.ResourceApiVersion = resource.DefaultAPIVersion
	source.ConfigSourceSpecifier = &core.ConfigSource_ApiConfigSource{
		ApiConfigSource: &core.ApiConfigSource{
			TransportApiVersion:       resource.DefaultAPIVersion,
			ApiType:                   core.ApiConfigSource_GRPC,
			SetNodeOnFirstMessageOnly: true,
			GrpcServices: []*core.GrpcService{{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: "xds_cluster"},
				},
			}},
		},
	}
	return source
}

func generateSnapshot(version int, services map[string]*EnvoyService) cache.Snapshot {
	// for each service create the endpoints
	edsEndpoints := make([]types.Resource, 0)
	edsClusters := make([]types.Resource, 0)
	for _, envoyService := range services {
		edsEndpoints = append(edsEndpoints, MakeEndpointsForService(envoyService))
		edsClusters = append(edsClusters, MakeClusterForService(envoyService))
	}

	if len(services) < 1 {
		panic("Unsupported: use a non-zero list of services: this dummy repo picks the first!")
	}
	var firstCluster string
	for _, service := range services {
		firstCluster = service.name
		break
	}

	return cache.NewSnapshot(
		fmt.Sprintf("%v.0", version),
		edsEndpoints, // endpoints
		edsClusters,  // clusters
		[]types.Resource{makeRoute("service_route", firstCluster, "target")},  // routes
		[]types.Resource{makeHTTPListener("listener_0", 80, "service_route")}, // listeners
		[]types.Resource{}, // runtimes
	)
}
