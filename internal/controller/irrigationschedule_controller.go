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

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	irrigatorv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
	"github.com/hauke-cloud/irrigator/internal/metrics"
	"github.com/hauke-cloud/irrigator/internal/scheduler"
)

const finalizerName = "irrigator.iot.hauke.cloud/finalizer"

// IrrigationScheduleReconciler reconciles IrrigationSchedule objects.
//
// +kubebuilder:rbac:groups=iot.hauke.cloud,resources=irrigationschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iot.hauke.cloud,resources=irrigationschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=iot.hauke.cloud,resources=irrigationschedules/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
type IrrigationScheduleReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Scheduler *scheduler.Scheduler
	Executor  *Executor
	Log       *slog.Logger
}

func (r *IrrigationScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With("schedule", req.NamespacedName)

	var schedule irrigatorv1alpha1.IrrigationSchedule
	if err := r.Get(ctx, req.NamespacedName, &schedule); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !schedule.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, req, &schedule)
	}

	if !controllerutil.ContainsFinalizer(&schedule, finalizerName) {
		controllerutil.AddFinalizer(&schedule, finalizerName)
		if err := r.Update(ctx, &schedule); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcileSchedule(ctx, req, &schedule, log)
}

func (r *IrrigationScheduleReconciler) handleDeletion(ctx context.Context, req ctrl.Request, schedule *irrigatorv1alpha1.IrrigationSchedule) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(schedule, finalizerName) {
		r.Scheduler.Remove(req.NamespacedName)
		controllerutil.RemoveFinalizer(schedule, finalizerName)
		if err := r.Update(ctx, schedule); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

func (r *IrrigationScheduleReconciler) reconcileSchedule(
	ctx context.Context,
	req ctrl.Request,
	schedule *irrigatorv1alpha1.IrrigationSchedule,
	log *slog.Logger,
) (ctrl.Result, error) {
	key := req.NamespacedName
	executor := r.Executor

	nextTime, err := r.Scheduler.AddOrUpdate(key, schedule.Spec, func() {
		execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if execErr := executor.Execute(execCtx, key); execErr != nil {
			log.ErrorContext(execCtx, "schedule execution failed", "error", execErr)
		}
	})
	if err != nil {
		r.Recorder.Eventf(schedule, "Warning", "InvalidCronExpression", "invalid cron expression: %v", err)
		metrics.ReconcileErrors.WithLabelValues(key.Name, key.Namespace).Inc()
		return r.setErrorCondition(ctx, schedule, err)
	}

	// Update active schedule count gauge.
	r.updateActiveGauge(ctx, schedule.Namespace)

	return r.setReadyCondition(ctx, schedule, nextTime)
}

func (r *IrrigationScheduleReconciler) setReadyCondition(ctx context.Context, schedule *irrigatorv1alpha1.IrrigationSchedule, nextTime time.Time) (ctrl.Result, error) {
	patch := client.MergeFrom(schedule.DeepCopy())

	msg := "schedule is disabled"
	if !nextTime.IsZero() {
		msg = fmt.Sprintf("next execution: %s", nextTime.Format(time.RFC3339))
		t := metav1.NewTime(nextTime)
		schedule.Status.NextExecutionTime = &t
	}

	apimeta.SetStatusCondition(&schedule.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Scheduled",
		Message:            msg,
		ObservedGeneration: schedule.Generation,
	})
	schedule.Status.ObservedGeneration = schedule.Generation

	if err := r.Status().Patch(ctx, schedule, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *IrrigationScheduleReconciler) setErrorCondition(ctx context.Context, schedule *irrigatorv1alpha1.IrrigationSchedule, reconcileErr error) (ctrl.Result, error) {
	patch := client.MergeFrom(schedule.DeepCopy())
	apimeta.SetStatusCondition(&schedule.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "InvalidConfiguration",
		Message:            reconcileErr.Error(),
		ObservedGeneration: schedule.Generation,
	})
	schedule.Status.ObservedGeneration = schedule.Generation

	if err := r.Status().Patch(ctx, schedule, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch error status: %w", err)
	}
	// No requeue — user must fix the spec.
	return ctrl.Result{}, nil
}

func (r *IrrigationScheduleReconciler) updateActiveGauge(ctx context.Context, namespace string) {
	var list irrigatorv1alpha1.IrrigationScheduleList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		r.Log.ErrorContext(ctx, "failed to list schedules for gauge", "namespace", namespace, "error", err)
		return
	}
	var count float64
	for i := range list.Items {
		if list.Items[i].Spec.Enabled {
			count++
		}
	}
	metrics.ActiveSchedules.WithLabelValues(namespace).Set(count)
}

func (r *IrrigationScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&irrigatorv1alpha1.IrrigationSchedule{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Complete(r)
}
