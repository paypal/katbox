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
	"errors"
	"math"
	"os"
	"path/filepath"

	"github.com/golang/glog"
)

// fullpath returns the location where the katbox volume will be created inside the container
// running the katbox plugin
func fullpath(workdir, podUUID, p string) string {
	return filepath.Join(workdir, podUUID, p)
}

// pressureFactor returns a value to be used with afterlife calculations.
// The more space we're using in the head room, the more aggressive the early eviction should be.
// Therefore, as the the ratio of headroom space available diminishes, our ratio gets lower and lower.
// Since we will be multiplying by a type time.Duration which is added to the deletion time, we multiply by
// the complement of the percentage of used headroom.
// e.g:
// 40% of our headroom is being used. Therefore we need to reduce the afterlife by 40% (use 60% of afterlife).
// e.g: 600 seconds * .60 = 360 seconds.
// Thus when we check if we should evict, the calculation will be done using 360 seconds after the delete
// happened instead of 600 after the delete happened.
// This concept is inspired by the Apache Mesos disk pressure feature.
func pressureFactor(total, free uint64, headroom float64) (float64, error) {
	if headroom < 0.0 || headroom > 1.0 {
		return 0, errors.New("headroom must be a value between 0 and 1.0 (inclusive)")
	}

	headroomSpace := uint64(math.Ceil(float64(total) * headroom))

	glog.V(5).Infof("Total Size: %v\nTotal Free: %v\nTotal Headroom Space: %v", total, free, headroomSpace)

	if free >= headroomSpace {
		return 1.0, nil
	}

	// Determines how far into the headroom space we currently are and returns the inverse as that's how
	// much of the afterlife we should be using.
	return 1.0 - float64(headroomSpace-free)/float64(headroomSpace), nil
}

// makeFile ensures that the file exists, creating it if necessary.
// The parent directory must exist.
func makeFile(pathname string) error {
	f, err := os.OpenFile(pathname, os.O_CREATE, os.FileMode(0644))
	defer f.Close()
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}
