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

const (
	deviceID           = "deviceID"
	maxStorageCapacity = tib
)

// Storage sizes
const (
	kib    int64 = 1024
	mib    int64 = kib * 1024
	gib    int64 = mib * 1024
	gib100 int64 = gib * 100
	tib    int64 = gib * 1024
	tib100 int64 = tib * 100
)

type accessType int

const (
	mountAccess accessType = iota
	blockAccess
)

// Available contexts for volume
const (
	podUUIDContext = "csi.storage.k8s.io/pod.uid"
	ephemeralContext = "csi.storage.k8s.io/ephemeral"
)

const (
	volumesBucketName = "volumes"
	deletedVolumesBucketName = "deletedVolumes"
)