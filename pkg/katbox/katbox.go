/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package katbox

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
)

type katbox struct {
	name          string
	nodeID        string
	version       string
	endpoint      string
	pruneInterval time.Duration
	headroom      float64

	idServer   *identityServer
	nodeServer *nodeServer
}

type volume struct {
	Name        string     `json:"name"`
	ID          string     `json:"id"`
	PodUUID     string     `json:"podUUID"`
	Size        int64      `json:"size"`
	Path        string     `json:"path"`
	AccessType  accessType `json:"accessType"`
	ParentVolID string     `json:"parentVolID,omitempty"`
	Ephemeral   bool       `json:"ephemeral"`
}

var (
	vendorVersion = "dev"
)

func NewKatboxDriver(
	driverName, nodeID, endpoint, workdir string,
	maxVolumesPerNode int64,
	afterlifeSpan, deleteInterval time.Duration,
	headroom float64,
	version string) (*katbox, error) {
	if driverName == "" {
		return nil, errors.New("no driver name provided")
	}

	if nodeID == "" {
		return nil, errors.New("no node id provided")
	}

	if endpoint == "" {
		return nil, errors.New("no driver endpoint provided")
	}
	if version != "" {
		vendorVersion = version
	}

	if err := os.MkdirAll(workdir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %v", err)
	}

	glog.Infof("Driver: %v ", driverName)
	glog.Infof("Version: %s", vendorVersion)

	return &katbox{
		name:          driverName,
		version:       vendorVersion,
		nodeID:        nodeID,
		endpoint:      endpoint,
		pruneInterval: deleteInterval,
		headroom:      headroom,
		idServer:      NewIdentityServer(driverName, version),
		nodeServer:    &nodeServer{node: NewNode(nodeID, workdir, maxVolumesPerNode, afterlifeSpan)},
	}, nil
}

func (k *katbox) Run() {
	if k.idServer == nil || k.nodeServer == nil || k.nodeServer.node == nil {
		glog.V(1).Infof("unable to create server")
		return
	}

	// Create GRPC servers
	s := NewNonBlockingGRPCServer()
	s.Start(k.endpoint, k.idServer, k.nodeServer)

	// Start pruner as a go routine
	endPrune := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(1)
	go k.nodeServer.node.deletedVolumes.periodicCleanup(endPrune, k.pruneInterval, &wg, k.headroom, k.nodeServer.node.workdir)

	// Wait for identity and node server to shut down
	s.Wait()

	// Signal to the pruner that it should clean up upon ending next loop
	close(endPrune)

	// Wait for pruner to signal that has finished cleaning up
	wg.Wait()
}
