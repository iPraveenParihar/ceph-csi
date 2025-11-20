/*
Copyright 2025 ceph-csi authors.

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

package healthchecker

import (
	"time"
)

type blockVolumeSizeChecker struct {
	checker

	// dirname points to the directory that is used for checking.
	dirname   string
	volumeId  string
	usageSize int64
}

func (b *blockVolumeSizeChecker) getUsageSize() (int64, error) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.usageSize, b.err
}

func newBlockVolumeSizeChecker(dir string, volumeId string) ConditionChecker {
	sc := &blockVolumeSizeChecker{
		dirname:  dir,
		volumeId: volumeId,
	}
	sc.initDefaults()

	sc.checker.runChecker = func() {
		sc.isRunning = true

		ticker := time.NewTicker(sc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-sc.commands: // STOP command received
				sc.isRunning = false

				return
			case now := <-ticker.C:
				sc.mutex.Lock()
				// TODO: getUsageSize()
				sc.lastUpdate = now
				sc.mutex.Unlock()
			}
		}
	}

	return sc
}
