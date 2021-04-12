/*
Copyright 2017 The Kubernetes Authors.

Modifications copyright 2020 PayPal.

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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"

	"golang.org/x/net/context"

	bolt "go.etcd.io/bbolt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/kubernetes/pkg/volume/util/volumepathhandler"
	"k8s.io/mount-utils"
)

const TopologyKeyNode = "topology.katbox.csi/node"

type nodeServer struct {
	node *node
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {

	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability missing in request")
	}
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "target path missing in request")
	}

	targetPath := req.GetTargetPath()
	// Kubernetes 1.15 doesn't have csi.storage.k8s.io/ephemeral.
	ephemeralVolume := req.GetVolumeContext()[ephemeralContext] == "true"
	podUUID := req.GetVolumeContext()[podUUIDContext]

	if !ephemeralVolume {
		return nil, status.Error(codes.InvalidArgument, "this CSI driver only supports ephemeral volumes")
	}

	if req.GetVolumeCapability().GetBlock() != nil &&
		req.GetVolumeCapability().GetMount() != nil {
		return nil, status.Error(codes.InvalidArgument, "volume cannot be of both block and mount access type")
	}

	volID := req.GetVolumeId()
	volName := fmt.Sprintf("ephemeral-%s", volID)
	ephVol, err := ns.node.createEphemeralVolume(req.GetVolumeId(), podUUID, volName, maxStorageCapacity, mountAccess)
	if err != nil && !os.IsExist(err) {
		glog.Error("failed to create ephemeral volume: ", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	glog.V(4).Infof("created ephemeral volume: %s", ephVol.Path)

	vol, err := ns.node.volumeByID(req.GetVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if req.GetVolumeCapability().GetBlock() != nil {
		if vol.AccessType != blockAccess {
			return nil, status.Error(codes.InvalidArgument, "cannot publish a non-block volume as block volume")
		}

		volPathHandler := volumepathhandler.VolumePathHandler{}

		// Get loop device from the volume path.
		loopDevice, err := volPathHandler.GetLoopDevice(vol.Path)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get the loop device: %v", err))
		}

		mounter := mount.New("")

		// Check if the target path exists. Create if not present.
		_, err = os.Lstat(targetPath)
		if os.IsNotExist(err) {
			if err = makeFile(targetPath); err != nil {
				return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create target path: %s: %v", targetPath, err))
			}
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to check if the target block file exists: %v", err)
		}

		// Check if the target path is already mounted. Prevent remounting.
		notMount, err := mount.IsNotMountPoint(mounter, targetPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, status.Errorf(codes.Internal, "error checking path %s for mount: %s", targetPath, err)
			}
			notMount = true
		}
		if !notMount {
			// It's already mounted.
			glog.V(5).Infof("Skipping bind-mounting subpath %s: already mounted", targetPath)
			return &csi.NodePublishVolumeResponse{}, nil
		}

		if err := mount.New("").Mount(loopDevice, targetPath, "", []string{"bind"}); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to mount block device: %s at %s: %v", loopDevice, targetPath, err))
		}
	} else if req.GetVolumeCapability().GetMount() != nil {
		if vol.AccessType != mountAccess {
			return nil, status.Error(codes.InvalidArgument, "cannot publish a non-mount volume as mount volume")
		}

		notMnt, err := mount.IsNotMountPoint(mount.New(""),targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				if err = os.MkdirAll(targetPath, 0750); err != nil {
					return nil, status.Error(codes.Internal, err.Error())
				}
				notMnt = true
			} else {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		if !notMnt {
			return &csi.NodePublishVolumeResponse{}, nil
		}

		fsType := req.GetVolumeCapability().GetMount().GetFsType()

		deviceId := ""
		if req.GetPublishContext() != nil {
			deviceId = req.GetPublishContext()[deviceID]
		}

		readOnly := req.GetReadonly()
		volumeId := req.GetVolumeId()
		attrib := req.GetVolumeContext()
		mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

		glog.V(4).Infof(
			"target %v\nfstype %v\ndevice %v\nreadonly %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
			targetPath,
			fsType,
			deviceId,
			readOnly,
			volumeId,
			attrib,
			mountFlags,
		)

		options := []string{"bind"}
		if readOnly {
			options = append(options, "ro")
		}
		mounter := mount.New("")
		volumePath := fullpath(ns.node.workdir, podUUID, volumeId)

		if err := mounter.Mount(volumePath, targetPath, "", options); err != nil {
			var errList strings.Builder
			errList.WriteString(err.Error())
			if vol.Ephemeral {
				if rmErr := os.RemoveAll(volumePath); rmErr != nil && !os.IsNotExist(rmErr) {
					errList.WriteString(fmt.Sprintf(" :%s", rmErr.Error()))
				}
			}
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to mount device: %s at %s: %s", volumePath, targetPath, errList.String()))
		}
	} else {
		return nil, status.Error(codes.InvalidArgument, "volume must be of block or mount access type")
	}

	// Persist newly created ephemeral volume into storage due to the fact that we need the PodUUID information
	// when deleting this object.
	err = ns.node.storage.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(volumesBucketName))
		if bucket == nil {
			return fmt.Errorf("bucket %s does not exist", volumesBucketName)
		}

		marshaledVol, err := json.Marshal(*ephVol)
		if err != nil {
			return fmt.Errorf("unable to serialize deletion candidate: %w", err)
		}

		err = bucket.Put([]byte(volID), marshaledVol)
		if err != nil {
			return fmt.Errorf("unable to insert volume %s into database: %w", volID, err)
		}

		return nil
	})

	if err != nil {
		glog.Errorf("Unable to persist volume %s: %s", volID, err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {

	// Validate the request that was sent
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "target path missing in request")
	}

	var vol volume
	var err error
	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()
	vol, err = ns.node.volumeByID(volumeID)

	if err != nil {
		glog.V(4).Infof("handling deletion for volume %v even though it was not found in memory", volumeID)
	} else if !vol.Ephemeral {
		glog.Warningf("handling deletion for volume %v even though it is not ephemeral", vol)
	}

	// Queue folder that was previously mounted on to pod for deletion. Note that this is different
	// than the point where the folder was bind mounted to.
	ns.node.deletedVolumes.queue(
		volumeID,
		deletionCandidate{
			Time:     time.Now(),
			Lifespan: ns.node.afterLifespan,
			Path:     fullpath(ns.node.workdir, vol.PodUUID, volumeID),
		},
	)

	// Unmount only if the target path is really a mount point.
	// This will not delete the underlying data stored in the working directory.
	notMnt, err := mount.IsNotMountPoint(mount.New(""), targetPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else if !notMnt {
		// Un-mounting the image or filesystem.
		err = mount.New("").Unmount(targetPath)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if vol.AccessType == blockAccess {
			return nil, fmt.Errorf("block access is unsupported by this driver")
	}

	// Delete persisted volume information
	err = ns.node.storage.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(volumesBucketName))

		err := bucket.Delete([]byte(volumeID))
		if err != nil {
			return fmt.Errorf("unable to delete %s from permanent storage: %s", volumeID, err)
		}
		return nil
	})

	if err != nil {
		glog.Error(err)
	}

	delete(ns.node.volumes, volumeID)

	// Since we've already successfully queued the local volume for deletion, we return
	// a payload indicating that the delete request was successful. The actual deletion from the local
	// disk will take place at a later time.
	// TODO(rdelvalle): Since we always tell k8s that we've been successful in removing the disk, we can run into
	// an inconsistency issue when we are unable to delete a volume. Explore options to handle this.
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capability missing in request")
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	topology := &csi.Topology{
		Segments: map[string]string{TopologyKeyNode: ns.node.id},
	}

	return &csi.NodeGetInfoResponse{
		NodeId:             ns.node.id,
		MaxVolumesPerNode:  ns.node.maxVolumes,
		AccessibleTopology: topology,
	}, nil
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{},
				},
			},
		},
	}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeExpandVolume is only implemented so the driver can be used for e2e testing.
func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
