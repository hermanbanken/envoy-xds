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
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	testv3 "github.com/envoyproxy/go-control-plane/pkg/test/v3"
)

var (
	l Logger

	port     uint
	basePort uint
	mode     string

	nodeID string
)

func init() {
	l = Logger{}

	flag.BoolVar(&l.Debug, "debug", false, "Enable xDS server debug logging")

	// The port that this xDS server listens on
	flag.UintVar(&port, "port", 1800, "xDS management server port")

	// Tell Envoy to use this Node ID
	flag.StringVar(&nodeID, "nodeID", "test-id", "Node ID")
}

func main() {
	flag.Parse()
	log.Println("Starting xDS server")

	// Create a cache
	cache := cachev3.NewSnapshotCache(false, cachev3.IDHash{}, l)

	// Set first snapshot
	snapshots := makeSnapshotChannel()
	snapshot := <-snapshots
	if err := cache.SetSnapshot(nodeID, snapshot); err != nil {
		l.Errorf("snapshot error %q for %+v", err, snapshot)
		os.Exit(1)
	}

	// Continuously monitor for new snapshots
	go func() {
		snapshot := <-snapshots
		if err := cache.SetSnapshot(nodeID, snapshot); err != nil {
			l.Errorf("snapshot error %q for %+v", err, snapshot)
			os.Exit(1)
		}
	}()

	// Run the xDS server
	ctx := context.Background()
	cb := &testv3.Callbacks{Debug: l.Debug}
	srv := serverv3.NewServer(ctx, cache, cb)
	runServer(ctx, srv, port)
}

func makeSnapshotChannel() chan cachev3.Snapshot {
	channel := make(chan cachev3.Snapshot, 1)
	version := 0
	MonitorServices(func(services map[string]*EnvoyService) {
		snapshot := generateSnapshot(version, services)
		if err := snapshot.Consistent(); err != nil {
			l.Errorf("snapshot inconsistency: %+v\n%+v", snapshot, err)
			os.Exit(1)
		}
		l.Debugf("will serve snapshot %+v", snapshot)
		channel <- snapshot
		version++
	})
	return channel
}

// MonitorServices is where you insert your custom discovery magic.
//
// For our demo, we just return a static list with 2 fields of metadata: "host" & "group"
//
// For a demonstration of using (multiple!) Kubernetes API servers, see
// https://github.com/envoyproxy/go-control-plane/blob/master/examples/dyplomat/main.go
//
func MonitorServices(handler func(map[string]*EnvoyService)) {
	// almost static, but you could do this based on some external system/event
	go func() {
		for {
			targets, err := lookup("target")
			if err != nil {
				log.Println(err)
			}
			endpoints := []EnvoyServiceEndpoint{}
			groups := []string{"a", "b"}
			for i, target := range targets {
				endpoints = append(endpoints, EnvoyServiceEndpoint{
					Address:  target,
					Metadata: map[string]string{"host": fmt.Sprintf("target-", i+1), "group": groups[i%2]},
				})
			}
			services := map[string]*EnvoyService{
				"service1": {
					name:      "service1",
					port:      8000,
					endpoints: endpoints,
				},
			}
			handler(services)
			time.Sleep(5 * time.Second)
		}
	}()
}

func lookup(addr string) ([]string, error) {
	dns := os.Getenv("DNS")
	if dns == "" {
		return net.DefaultResolver.LookupHost(context.Background(), addr)
	}
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", dns)
		},
	}
	return r.LookupHost(context.Background(), addr)

}
