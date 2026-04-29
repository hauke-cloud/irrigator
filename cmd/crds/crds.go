/*
Copyright 2026 hauke.cloud.

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

package crds

import (
	_ "embed"
)

// Embed CRD manifests
//
//go:embed iot.hauke.cloud_schedules.yaml
var ScheduleCRD []byte

// GetAll returns all CRD manifests that should be installed by this controller
// Note: MQTTBridge and Device CRDs are managed by external controller
func GetAll() [][]byte {
	return [][]byte{
		ScheduleCRD,
	}
}
