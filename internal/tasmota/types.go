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

import "time"

// MQTTPublisher is the interface for publishing MQTT commands
type MQTTPublisher interface {
	PublishTasmotaCommand(namespace, bridgeName, command, payload string) error
}

// TelemetryMessage represents a Tasmota telemetry message
type TelemetryMessage struct {
	Time       string                  `json:"Time"`
	ZbReceived map[string]ZigbeeDevice `json:"ZbReceived,omitempty"`
}

// ZigbeeDevice represents a Zigbee device in the ZbReceived payload
type ZigbeeDevice struct {
	Device            string   `json:"Device"`
	Name              string   `json:"Name,omitempty"`
	Power             *int     `json:"Power,omitempty"`
	Endpoint          *int     `json:"Endpoint,omitempty"`
	LinkQuality       *int     `json:"LinkQuality,omitempty"`
	BatteryPercentage *int     `json:"BatteryPercentage,omitempty"`
	Temperature       *float64 `json:"Temperature,omitempty"`
	Humidity          *float64 `json:"Humidity,omitempty"`
	Pressure          *float64 `json:"Pressure,omitempty"`
	Voltage           *float64 `json:"Voltage,omitempty"`
	Contact           *bool    `json:"Contact,omitempty"`
	Occupancy         *bool    `json:"Occupancy,omitempty"`
	WaterLeak         *bool    `json:"WaterLeak,omitempty"`
	// Additional fields stored as raw data
	Additional map[string]any `json:"-"`
}

// StatusMessage represents a Tasmota status/result message
type StatusMessage struct {
	ZbSend    *ZbSendResult          `json:"ZbSend,omitempty"`
	ZbName    *ZbNameResult          `json:"ZbName,omitempty"`
	ZbStatus1 []ZbStatus1DeviceEntry `json:"ZbStatus1,omitempty"`
	ZbStatus3 []ZbStatus3DeviceEntry `json:"ZbStatus3,omitempty"`
}

// ZbSendResult represents the result of a ZbSend command
type ZbSendResult struct {
	Device   string `json:"Device"`
	Status   string `json:"Status,omitempty"`
	Endpoint *int   `json:"Endpoint,omitempty"`
}

// ZbNameResult represents the result of a ZbName command
type ZbNameResult struct {
	Device string `json:"Device"`
	Name   string `json:"Name"`
}

// ZbStatus1DeviceEntry represents a device entry in ZbStatus1 response
type ZbStatus1DeviceEntry struct {
	Device string `json:"Device"` // IEEE address (e.g., "0x4F2E")
	Name   string `json:"Name"`   // Friendly name
}

// ZbStatus3DeviceEntry represents a device entry in ZbStatus3 response
type ZbStatus3DeviceEntry struct {
	Device               string   `json:"Device"`                         // Short address (e.g., "0x4F2E")
	Name                 string   `json:"Name"`                           // Friendly name
	IEEEAddr             string   `json:"IEEEAddr"`                       // Full 64-bit IEEE address (e.g., "0xF4B3B1FFFE4EA459")
	ModelId              string   `json:"ModelId,omitempty"`              // Device model
	Manufacturer         string   `json:"Manufacturer,omitempty"`         // Manufacturer
	Endpoints            []int    `json:"Endpoints,omitempty"`            // Available endpoints
	Config               []string `json:"Config,omitempty"`               // Device configuration
	Power                *int     `json:"Power,omitempty"`                // Power state
	Reachable            *bool    `json:"Reachable,omitempty"`            // Device reachability
	BatteryPercentage    *int     `json:"BatteryPercentage,omitempty"`    // Battery level 0-100
	BatteryLastSeenEpoch *int64   `json:"BatteryLastSeenEpoch,omitempty"` // Last battery update timestamp
	LastSeen             *int     `json:"LastSeen,omitempty"`             // Seconds since last seen
	LastSeenEpoch        *int64   `json:"LastSeenEpoch,omitempty"`        // Last seen Unix timestamp
	LinkQuality          *int     `json:"LinkQuality,omitempty"`          // Link quality 0-255
}

// StateMessage represents a Tasmota state message
type StateMessage struct {
	Time   string      `json:"Time"`
	Uptime string      `json:"Uptime,omitempty"`
	Wifi   *WifiStatus `json:"Wifi,omitempty"`
	Heap   *int        `json:"Heap,omitempty"`
}

// WifiStatus represents WiFi connection status
type WifiStatus struct {
	RSSI   int `json:"RSSI,omitempty"`
	Signal int `json:"Signal,omitempty"`
}

// InfoMessage represents a Tasmota info message
type InfoMessage struct {
	Version       string `json:"Version,omitempty"`
	Module        string `json:"Module,omitempty"`
	FallbackTopic string `json:"FallbackTopic,omitempty"`
	GroupTopic    string `json:"GroupTopic,omitempty"`
}

// MessageContext holds context information for message processing
type MessageContext struct {
	BridgeName      string
	BridgeNamespace string
	Topic           string
	Timestamp       time.Time
}
