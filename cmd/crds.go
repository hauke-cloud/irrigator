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

package main

import (
	_ "embed"
)

// Embed CRD YAML files into the binary
// These files are generated from kubernetes-iot-api types

// Note: MQTTBridge and Device CRDs are NOT embedded or installed by this operator.
// These CRDs are managed by the external device management controller.
// This operator only manages the Schedule CRD.

//go:embed crd/iot.hauke.cloud_schedules.yaml
var scheduleCRD string
