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

package controller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	irrigatorv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
	"github.com/hauke-cloud/irrigator/internal/metrics"
	"github.com/hauke-cloud/irrigator/internal/scheduler"
	"github.com/hauke-cloud/irrigator/internal/valve"
)

// Executor carries out an IrrigationSchedule by calling the valve-controller API
// and patching the schedule's status. It is safe to call from cron goroutines and
// REST handlers concurrently.
type Executor struct {
	k8s         client.Client
	valveClient *valve.Client
	sched       *scheduler.Scheduler
	log         *slog.Logger
}

// NewExecutor creates an Executor.
func NewExecutor(k8s client.Client, valveClient *valve.Client, sched *scheduler.Scheduler, log *slog.Logger) *Executor {
	return &Executor{k8s: k8s, valveClient: valveClient, sched: sched, log: log}
}

// Execute fetches the latest schedule spec and either opens the valve or logs
// a dry-run. Status is patched afterwards regardless of outcome.
func (e *Executor) Execute(ctx context.Context, key types.NamespacedName) error {
	var schedule irrigatorv1alpha1.IrrigationSchedule
	if err := e.k8s.Get(ctx, key, &schedule); err != nil {
		return client.IgnoreNotFound(fmt.Errorf("get schedule %s: %w", key, err))
	}

	if !schedule.Spec.Enabled {
		e.log.InfoContext(ctx, "schedule disabled, skipping execution", "schedule", key)
		return nil
	}

	result := "success"
	var execErr error

	if schedule.Spec.DryRun {
		e.log.InfoContext(ctx, "dry-run: would open valve",
			"schedule", key,
			"valve", schedule.Spec.ValveRef.Name,
			"duration_s", schedule.Spec.DurationSeconds,
		)
		result = "dry-run"
	} else {
		ns := schedule.Spec.ValveRef.Namespace
		if ns == "" {
			ns = key.Namespace
		}
		duration := time.Duration(schedule.Spec.DurationSeconds) * time.Second
		_, execErr = e.valveClient.OpenValve(ctx, schedule.Spec.ValveRef.Name, ns, duration)
		if execErr != nil {
			e.log.ErrorContext(ctx, "failed to open valve",
				"schedule", key,
				"valve", schedule.Spec.ValveRef.Name,
				"error", execErr,
			)
			result = "failed"
			metrics.ScheduleValveErrors.WithLabelValues(key.Name, key.Namespace, schedule.Spec.ValveRef.Name).Inc()
		}

		metrics.ValveOpenDurationSeconds.
			WithLabelValues(key.Name, key.Namespace, schedule.Spec.ValveRef.Name).
			Observe(float64(schedule.Spec.DurationSeconds))
	}

	metrics.ScheduleExecutions.WithLabelValues(key.Name, key.Namespace, schedule.Spec.ValveRef.Name, result).Inc()
	metrics.ScheduleLastExecutionTimestamp.
		WithLabelValues(key.Name, key.Namespace, schedule.Spec.ValveRef.Name).
		SetToCurrentTime()

	e.patchStatus(ctx, key, &schedule, result)

	return execErr
}

func (e *Executor) patchStatus(ctx context.Context, key types.NamespacedName, schedule *irrigatorv1alpha1.IrrigationSchedule, result string) {
	patch := client.MergeFrom(schedule.DeepCopy())

	now := metav1.Now()
	schedule.Status.LastExecutionTime = &now
	schedule.Status.LastExecutionResult = result
	schedule.Status.ExecutionCount++

	if next, err := e.sched.NextTime(key); err == nil {
		t := metav1.NewTime(next)
		schedule.Status.NextExecutionTime = &t
		metrics.ScheduleNextExecutionTimestamp.
			WithLabelValues(key.Name, key.Namespace, schedule.Spec.ValveRef.Name).
			Set(float64(next.Unix()))
	}

	if err := e.k8s.Status().Patch(ctx, schedule, patch); err != nil {
		e.log.ErrorContext(ctx, "failed to patch schedule status", "schedule", key, "error", err)
	}
}
