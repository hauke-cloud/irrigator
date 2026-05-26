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

import (
	"sync"
	"time"
)

// ConfirmationResult holds the outcome of a valve state confirmation.
type ConfirmationResult struct {
	ActualPower int
	Confirmed   bool
}

type confirmationEntry struct {
	expectedPower int
	result        chan ConfirmationResult
	registered    time.Time
}

// ConfirmationRegistry tracks pending valve power-state confirmations.
// Callers register before sending a command so that any racing telemetry
// message is never dropped.
type ConfirmationRegistry struct {
	mu      sync.Mutex
	pending map[string]*confirmationEntry // key: IEEE address
}

// NewConfirmationRegistry creates an empty ConfirmationRegistry.
func NewConfirmationRegistry() *ConfirmationRegistry {
	return &ConfirmationRegistry{
		pending: make(map[string]*confirmationEntry),
	}
}

// Register records an expected power state for ieeeAddr and returns a
// buffered channel that will receive exactly one result when Notify is called.
// The caller must call Deregister if the channel is no longer needed.
func (r *ConfirmationRegistry) Register(ieeeAddr string, expectedPower int) <-chan ConfirmationResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan ConfirmationResult, 1)
	r.pending[ieeeAddr] = &confirmationEntry{
		expectedPower: expectedPower,
		result:        ch,
		registered:    time.Now(),
	}
	return ch
}

// Notify resolves a pending confirmation for ieeeAddr with the observed
// actualPower. Returns true when a matching entry was found.
func (r *ConfirmationRegistry) Notify(ieeeAddr string, actualPower int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.pending[ieeeAddr]
	if !ok {
		return false
	}
	delete(r.pending, ieeeAddr)
	select {
	case entry.result <- ConfirmationResult{
		ActualPower: actualPower,
		Confirmed:   actualPower == entry.expectedPower,
	}:
	default:
	}
	return true
}

// Deregister removes any pending confirmation for ieeeAddr without sending
// a result. Safe to call even when no entry exists.
func (r *ConfirmationRegistry) Deregister(ieeeAddr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pending, ieeeAddr)
}
