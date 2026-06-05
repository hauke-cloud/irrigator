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

// Package scheduler wraps robfig/cron to manage per-schedule cron entries with
// per-entry timezone support and safe concurrent add/remove operations.
package scheduler

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/types"

	irrigatorv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
)

var ErrNotFound = errors.New("cron entry not found")

// Scheduler manages a pool of per-IrrigationSchedule cron entries.
type Scheduler struct {
	cron    *cron.Cron
	entries map[string]cron.EntryID
	mu      sync.RWMutex
	log     *slog.Logger
}

// New starts a new Scheduler. Call Stop when done.
func New(log *slog.Logger) *Scheduler {
	c := cron.New()
	c.Start()
	return &Scheduler{
		cron:    c,
		entries: make(map[string]cron.EntryID),
		log:     log,
	}
}

// AddOrUpdate registers or replaces the cron entry for key using the given spec.
// If spec.Enabled is false, any existing entry is removed and (time.Time{}, nil) is returned.
// Returns the next scheduled execution time.
func (s *Scheduler) AddOrUpdate(key types.NamespacedName, spec irrigatorv1alpha1.IrrigationScheduleSpec, job func()) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key.String()

	if id, ok := s.entries[k]; ok {
		s.cron.Remove(id)
		delete(s.entries, k)
	}

	if !spec.Enabled {
		s.log.Info("schedule disabled, no cron entry registered", "key", k)
		return time.Time{}, nil
	}

	tz := spec.TimeZone
	if tz == "" {
		tz = "Europe/Berlin"
	}
	// cron v3 supports the TZ= prefix to set the timezone per entry.
	cronSpec := fmt.Sprintf("TZ=%s %s", tz, spec.CronExpression)

	id, err := s.cron.AddFunc(cronSpec, job)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q (tz=%s): %w", spec.CronExpression, tz, err)
	}
	s.entries[k] = id

	next := s.cron.Entry(id).Next
	s.log.Info("cron entry registered", "key", k, "spec", cronSpec, "next", next)
	return next, nil
}

// Remove removes the cron entry for key. Safe to call when no entry exists.
func (s *Scheduler) Remove(key types.NamespacedName) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key.String()
	if id, ok := s.entries[k]; ok {
		s.cron.Remove(id)
		delete(s.entries, k)
		s.log.Info("cron entry removed", "key", k)
	}
}

// NextTime returns the next scheduled execution time for key.
func (s *Scheduler) NextTime(key types.NamespacedName) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.entries[key.String()]
	if !ok {
		return time.Time{}, ErrNotFound
	}
	return s.cron.Entry(id).Next, nil
}

// Stop halts the cron runner and returns a context that is done when all
// in-progress jobs have completed.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}
