/*
Copyright 2020 PayPal.

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
	"path/filepath"
	"sync"
	"time"

	"github.com/ricochet2200/go-disk-usage/du"
	bolt "go.etcd.io/bbolt"

	"github.com/golang/glog"
)

type deletedVolumes struct {
	candidates map[string]*deletionCandidate
	storage    *bolt.DB
	lock       sync.RWMutex
}

type deletionCandidate struct {
	Time     time.Time     `json:"deleteTime"`
	Lifespan time.Duration `json:"lifespan"`
	Path     string        `json:"path"`
}

func (d *deletedVolumes) periodicCleanup(
	done <-chan struct{},
	interval time.Duration,
	wg *sync.WaitGroup,
	headroom float64,
	workdir string,
) {
	for {
		select {
		case <-done:
			if err := d.storage.Close(); err != nil {
				glog.Info("unable to close persistent storage ", err)
			}
			wg.Done()
			return
		default:
			d.prune(workdir, headroom)
			time.Sleep(interval)
		}
	}
}

func (d *deletedVolumes) queue(id string, vol deletionCandidate) {
	// Check if an entry for deletion already exists
	d.lock.RLock()
	_, found := d.candidates[id]
	d.lock.RUnlock()

	if found {
		return
	}

	// Write ahead persist to local storage the volume that will be entering our deletion queue
	err := d.storage.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(deletedVolumesBucketName))
		if bucket == nil {
			return fmt.Errorf("bucket %s does not exist", deletedVolumesBucketName)
		}

		marshaledVol, err := json.Marshal(vol)
		if err != nil {
			return fmt.Errorf("unable to serialize deletion candidate: %w", err)
		}

		err = bucket.Put([]byte(id), marshaledVol)
		if err != nil {
			return fmt.Errorf("unable to insert volume into database: %w", err)
		}

		return nil
	})

	if err != nil {
		glog.Infof("failed to persist "+id+" at "+vol.Path, ": ", err)
		return
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	d.candidates[id] = &vol
}

func (d *deletedVolumes) remove(id string) {
	err := d.storage.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(deletedVolumesBucketName))

		err := bucket.Delete([]byte(id))
		if err != nil {
			return fmt.Errorf("unable to delete %s from permanent storage: %s", id, err)
		}
		return nil
	})

	if err != nil {
		glog.Infof("failed to remove "+id+": ", err)
		return
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	delete(d.candidates, id)
}

func (d *deletedVolumes) prune(workdir string, headroom float64) {
	glog.V(2).Info("number of volumes queued for deletion: ", len(d.candidates))

	// Only get the time once since this results in a syscall
	// This may mean that some volumes may need to wait until next cycle to be pruned
	currentTime := time.Now()

	// Determine pressure factor based on underlying storage utilization
	diskUsage := du.NewDiskUsage(workdir)
	pressureFactor, err := pressureFactor(diskUsage.Size(), diskUsage.Free(), headroom)
	if err != nil {
		glog.Info("error calculating pressure factor, setting pressure factor to default value of 0.10 ", err)
		pressureFactor = 0.1
	}

	glog.Info("disk pressure factor being used for this prune round: ", pressureFactor)

	// Create a deep copy of the maps for safe reading
	candidatesCopy := make(map[string]*deletionCandidate)
	d.lock.RLock()
	for id, vol := range d.candidates {
		candidatesCopy[id] = vol
	}
	d.lock.RUnlock()

	// Iterate over the copy of the candidates list since iterating over the original
	// provides no concurrency safety and attempting to use locks leads to a deadlock in many code paths.
	for id, vol := range candidatesCopy {
		if vol == nil {
			continue
		}

		if glog.V(5) {
			glog.Infof("Deletion candidate volume ID: %v\n%+v", id, vol)
		}

		// Short circuit if the path doesn't exist
		if _, err := os.Stat(vol.Path); os.IsNotExist(err) {
			glog.Infof("removing %v from queue as path %v does not exist", id, vol.Path)
			d.remove(id)
		}

		// Check to see if the current has passed the time when we need to evict this volume from
		// the underlying storage. The point in time is a combination of the pressure factor
		// and the configured afterlife duration.
		if currentTime.After(vol.Time.Add(time.Duration(float64(vol.Lifespan) * pressureFactor))) {
			err := os.RemoveAll(vol.Path)
			if err != nil {
				glog.Infof("unable to delete "+id+" at "+vol.Path, ": ", err)
				continue
			}

			// Attempt to remove PodUUID directory if empty.
			// We ignore the error here because this will correctly fail when a pod with multiple katbox volumes
			// attempts to delete the parent directory. Only the last remaining volume being deleted should succeed.
			_ = os.Remove(filepath.Dir(vol.Path))

			glog.Infof("deleted " + id + " at " + vol.Path)
			d.remove(id)
		}
	}
}
