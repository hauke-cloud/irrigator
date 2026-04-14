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

package tasmota

import "strings"

// sanitizeDeviceName converts an IEEE address or device name to a valid Kubernetes resource name
func sanitizeDeviceName(name string) string {
	// Remove "0x" prefix if present
	name = strings.TrimPrefix(strings.ToLower(name), "0x")

	// Replace invalid characters with hyphens
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Ensure it starts with alphanumeric
	if len(name) > 0 && (name[0] < 'a' || name[0] > 'z') && (name[0] < '0' || name[0] > '9') {
		name = "device-" + name
	}

	// Add prefix to make it more readable
	if !strings.HasPrefix(name, "device-") {
		name = "device-" + name
	}

	// Limit length (Kubernetes resource names max 253 chars)
	if len(name) > 253 {
		name = name[:253]
	}

	return name
}

// sanitizeLabel converts a value to a valid Kubernetes label value
func sanitizeLabel(value string) string {
	// Remove "0x" prefix
	value = strings.TrimPrefix(strings.ToLower(value), "0x")

	// Labels must be alphanumeric, '-', '_', or '.'
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, value)

	// Max length 63 characters
	if len(value) > 63 {
		value = value[:63]
	}

	return value
}
