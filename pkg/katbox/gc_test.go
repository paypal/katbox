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
	"sync"
	"testing"
	"time"
)

func TestDelete(t *testing.T) {
	deleteQueue := deletedVolumes{candidates: make(map[string]*deletionCandidate), lock: sync.RWMutex{}}
	deleteQueue.queue("volume1", deletionCandidate{
		Time:     time.Now(),
		Lifespan: time.Second * 5,
		Path:     "/doesnt/exist",
	})
	deleteQueue.queue("vol2", deletionCandidate{
		Time:     time.Now(),
		Lifespan: time.Second * 1,
		Path:     "/doesnt/exist2",
	})

	deleteQueue.queue("vol3", deletionCandidate{
		Time:     time.Now(),
		Lifespan: time.Second * 3,
		Path:     "/doesnt/exist3",
	})

	for len(deleteQueue.candidates) > 0 {
		deleteQueue.prune()
		time.Sleep(time.Second * 1)
	}
}
