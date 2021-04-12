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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/golang/glog"

	bolt "go.etcd.io/bbolt"

	"k8s.io/kubernetes/pkg/volume/util/volumepathhandler"
	utilexec "k8s.io/utils/exec"
)

type node struct {
	id             string
	volumes        map[string]volume
	deletedVolumes deletedVolumes
	workdir        string
	afterLifespan  time.Duration
	maxVolumes     int64
	storage        *bolt.DB
}

func NewNode(id, workdir string, maxVolumes int64, afterLifespan time.Duration) *node {
	db, err := initializePermanentStorage(
		path.Join(workdir, "deletedVolumes.db"),
		deletedVolumesBucketName,
		volumesBucketName)
	if err != nil {
		return nil
	}

	candidates, err := loadDeletedVolumesFromPersistent(db, deletedVolumesBucketName)
	if err != nil {
		return nil
	}

	volumes, err := loadVolumesFromPersistent(db, volumesBucketName)
	if err != nil {
		return nil
	}

	glog.V(4).Infof("loaded %d volume records into memory", len(volumes))

	return &node{
		id:      id,
		volumes: volumes,
		deletedVolumes: deletedVolumes{
			candidates: candidates,
			lock:       sync.RWMutex{},
			storage:    db,
		},
		workdir:       workdir,
		afterLifespan: afterLifespan,
		maxVolumes:    maxVolumes,
		storage:       db,
	}
}

func initializePermanentStorage(dbFilename string, bucketNames ...string) (*bolt.DB, error) {
	db, err := bolt.Open(dbFilename, 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		glog.V(4).Infof("unable to open persistent storage: %s", err)
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range bucketNames {
			_, err := tx.CreateBucketIfNotExists([]byte(name))
			if err != nil {
				return fmt.Errorf("unable to create bucket: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		glog.V(4).Infof("unable to create bucket for storage: %s", err)
		return nil, err
	}

	return db, nil
}

func loadDeletedVolumesFromPersistent(db *bolt.DB, bucketName string) (map[string]*deletionCandidate, error) {
	if db == nil {
		return nil, errors.New("database has not been initialized")
	}

	// Load list of volumes to be deleted from the persistent layer
	candidates := make(map[string]*deletionCandidate)
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return fmt.Errorf("bucket %s doesn't exist", bucketName)
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var candidate deletionCandidate
			id := string(k)
			if err := json.Unmarshal(v, &candidate); err != nil {
				glog.V(1).Infof("unable to load volume %s: %s", id, err)
			} else {
				glog.V(4).Infof("loaded volume %s info into memory", id)
			}

			candidates[id] = &candidate
		}

		return nil
	})

	if err != nil {
		glog.Info("unable load persistent layer into memory ", err)
		return nil, err
	}

	return candidates, nil
}

func loadVolumesFromPersistent(db *bolt.DB, bucketName string) (map[string]volume, error) {
	if db == nil {
		return nil, errors.New("database has not been initialized")
	}

	// Load list of volumes to be deleted from the persistent layer
	volumes := make(map[string]volume)
	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return fmt.Errorf("bucket %s doesn't exist", bucketName)
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var vol volume
			id := string(k)
			if err := json.Unmarshal(v, &vol); err != nil {
				glog.V(1).Infof("unable to load volume %s: %s", id, err)
			} else {
				glog.V(4).Infof("loaded volume %s info into memory", id)
			}

			volumes[id] = vol
		}
		return nil
	})

	if err != nil {
		glog.Info("unable load persistent layer into memory: ", err)
		return nil, err
	}

	return volumes, nil
}

// createEphemeralVolume create the directory for the katbox volume.
// It returns the volume path or err if one occurs.
func (n *node) createEphemeralVolume(volID, podUUID, name string, cap int64, volAccessType accessType) (*volume, error) {
	fullPath := fullpath(n.workdir, podUUID, volID)

	switch volAccessType {
	case mountAccess:
		err := os.MkdirAll(fullPath, 0777)
		if err != nil {
			return nil, err
		}
	case blockAccess:
		executor := utilexec.New()
		size := fmt.Sprintf("%dM", cap/mib)
		// Create a block file.
		out, err := executor.Command("fallocate", "-l", size, fullPath).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to create block device: %v, %v", err, string(out))
		}

		// Associate block file with the loop device.
		volPathHandler := volumepathhandler.VolumePathHandler{}
		_, err = volPathHandler.AttachFileDevice(fullPath)
		if err != nil {
			// Remove the block file because it'll no longer be used again.
			if err2 := os.Remove(fullPath); err2 != nil {
				glog.Errorf("failed to cleanup block file %s: %v", fullPath, err2)
			}
			return nil, fmt.Errorf("failed to attach device %v: %v", fullPath, err)
		}
	default:
		return nil, fmt.Errorf("unsupported access type %v", volAccessType)
	}

	vol := volume{
		Name:       name,
		ID:         volID,
		PodUUID:    podUUID,
		Size:       cap,
		Path:       fullPath,
		AccessType: volAccessType,
		Ephemeral:  true,
	}
	n.volumes[volID] = vol
	return &vol, nil
}

func (n *node) volumeByID(id string) (volume, error) {
	if vol, ok := n.volumes[id]; ok {
		return vol, nil
	}
	return volume{}, fmt.Errorf("volume id %s does not exist in the volumes list", id)
}
