/*
Copyright 2025 Hauke Mettendorf.

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

package valve

import "time"

type Valve struct {
	Name                    string     `json:"name"`
	Namespace               string     `json:"namespace"`
	State                   string     `json:"state"`
	Disabled                bool       `json:"disabled"`
	FriendlyName            string     `json:"friendlyName"`
	BridgeName              string     `json:"bridgeName"`
	LinkQuality             int32      `json:"linkQuality"`
	BatteryPercentage       int32      `json:"batteryPercentage"`
	Reachable               bool       `json:"reachable"`
	DailyIrrigationVolume   float64    `json:"dailyIrrigationVolume"`
	LastValveOpenDurationMs int64      `json:"lastValveOpenDurationMs"`
	LastOpenTime            *time.Time `json:"lastOpenTime,omitempty"`
	LastCloseTime           *time.Time `json:"lastCloseTime,omitempty"`
	CurrentAction           *Action    `json:"currentAction,omitempty"`
}

type Action struct {
	ID          string     `json:"id"`
	Valve       string     `json:"valve"`
	Type        string     `json:"type"`
	State       string     `json:"state"`
	Duration    string     `json:"duration,omitempty"`
	RequestedAt time.Time  `json:"requestedAt"`
	FulfilledAt *time.Time `json:"fulfilledAt,omitempty"`
	ClosedAt    *time.Time `json:"closedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	FailedAt    *time.Time `json:"failedAt,omitempty"`
	Attempts    int        `json:"attempts"`
	Error       string     `json:"error,omitempty"`
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}
