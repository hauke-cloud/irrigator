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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ScheduleExecutions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "irrigator_schedule_executions_total",
		Help: "Total number of schedule executions by result.",
	}, []string{"schedule", "namespace", "valve", "result"})

	ScheduleValveErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "irrigator_schedule_valve_errors_total",
		Help: "Total number of valve command errors during schedule execution.",
	}, []string{"schedule", "namespace", "valve"})

	ScheduleLastExecutionTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "irrigator_schedule_last_execution_timestamp_seconds",
		Help: "Unix timestamp of the last schedule execution.",
	}, []string{"schedule", "namespace", "valve"})

	ScheduleNextExecutionTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "irrigator_schedule_next_execution_timestamp_seconds",
		Help: "Unix timestamp of the next scheduled execution.",
	}, []string{"schedule", "namespace", "valve"})

	ActiveSchedules = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "irrigator_active_schedules",
		Help: "Number of enabled irrigation schedules.",
	}, []string{"namespace"})

	ValveOpenDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "irrigator_valve_open_duration_seconds",
		Help:    "Configured open duration per schedule execution.",
		Buckets: prometheus.LinearBuckets(30, 30, 12),
	}, []string{"schedule", "namespace", "valve"})

	ReconcileErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "irrigator_reconcile_errors_total",
		Help: "Total number of reconcile errors.",
	}, []string{"schedule", "namespace"})
)
