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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mqttv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
	"github.com/hauke-cloud/irrigator/internal/tasmota"
)

// ScheduleReconciler reconciles a Schedule object
type ScheduleReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Log             *zap.Logger
	ValveController *tasmota.ValveController
}

// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=schedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=schedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=schedules/finalizers,verbs=update
// +kubebuilder:rbac:groups=mqtt.hauke.cloud,resources=devices,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(
		zap.String("namespace", req.Namespace),
		zap.String("name", req.Name))

	log.Debug("Reconciling Schedule")

	// Fetch the Schedule instance
	schedule := &mqttv1alpha1.Schedule{}
	err := r.Get(ctx, req.NamespacedName, schedule)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debug("Schedule not found, likely deleted")
			return ctrl.Result{}, nil
		}
		log.Error("Failed to get Schedule", zap.Error(err))
		return ctrl.Result{}, err
	}

	// Parse cron schedule early to set next run time even if schedule is disabled or has errors
	cronSchedule, loc, nextRun, cronErr := r.parseCronScheduleEarly(schedule, log)
	if cronErr == nil {
		// Update next scheduled time
		nextRunTime := metav1.NewTime(nextRun)
		schedule.Status.NextScheduledTime = &nextRunTime
		// Format the time in the configured timezone
		schedule.Status.NextScheduledTimeFormatted = nextRun.In(loc).Format("2006-01-02 15:04:05 MST")

		// Update last execution time formatted if we have a last execution time
		if schedule.Status.LastExecutionTime != nil {
			schedule.Status.LastExecutionTimeFormatted = schedule.Status.LastExecutionTime.Time.In(loc).Format("2006-01-02 15:04:05 MST")
		}
	}

	// Check if schedule is enabled
	if schedule.Spec.Enabled != nil && !*schedule.Spec.Enabled {
		return r.handleDisabledSchedule(ctx, schedule, log)
	}

	// If cron parsing failed, return error
	if cronErr != nil {
		schedule.Status.Message = fmt.Sprintf("Invalid cron expression: %s", cronErr.Error())
		if statusErr := r.Status().Update(ctx, schedule); statusErr != nil {
			log.Error("Failed to update status", zap.Error(statusErr))
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, cronErr
	}

	// Update valve controller dry-run mode based on schedule spec
	r.ValveController.SetDryRun(schedule.Spec.DryRun)

	// Find and validate the device
	device, result, err := r.findAndValidateDevice(ctx, schedule, log)
	if err != nil {
		return result, err
	}

	// Update resolved device name and power state
	r.updateDeviceStatus(schedule, device, log)

	// Check if we should execute now
	shouldExecute := r.shouldExecuteNow(schedule, cronSchedule, loc)

	// Handle irrigation execution
	if err := r.handleIrrigationExecution(ctx, schedule, device, shouldExecute, loc, log); err != nil {
		log.Error("Error during irrigation execution", zap.Error(err))
	}

	// Handle irrigation stop
	if err := r.handleIrrigationStop(ctx, schedule, device, loc, log); err != nil {
		log.Error("Error during irrigation stop", zap.Error(err))
	}

	// Update status
	if err := r.Status().Update(ctx, schedule); err != nil {
		log.Error("Failed to update status", zap.Error(err))
		return ctrl.Result{}, err
	}

	// Requeue based on state
	return r.determineRequeueInterval(schedule), nil
}

// handleDisabledSchedule handles the case where a schedule is disabled
func (r *ScheduleReconciler) handleDisabledSchedule(ctx context.Context, schedule *mqttv1alpha1.Schedule, log *zap.Logger) (ctrl.Result, error) {
	log.Debug("Schedule is disabled, skipping")
	schedule.Status.Message = "Schedule is disabled"
	if err := r.Status().Update(ctx, schedule); err != nil {
		log.Error("Failed to update status", zap.Error(err))
	}
	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

// findAndValidateDevice finds the device and validates it's a valve
func (r *ScheduleReconciler) findAndValidateDevice(ctx context.Context, schedule *mqttv1alpha1.Schedule, log *zap.Logger) (*mqttv1alpha1.Device, ctrl.Result, error) {
	device, err := r.findDevice(ctx, schedule)
	if err != nil {
		log.Error("Failed to find device", zap.Error(err))
		schedule.Status.Message = fmt.Sprintf("Device not found: %s", err.Error())
		if statusErr := r.Status().Update(ctx, schedule); statusErr != nil {
			log.Error("Failed to update status", zap.Error(statusErr))
		}
		return nil, ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	log.Debug("Found device for schedule",
		zap.String("deviceName", device.Name),
		zap.String("ieeeAddr", device.Spec.IEEEAddr))

	// Verify device is a valve
	if device.Spec.SensorType != "valve" {
		log.Warn("Device is not a valve",
			zap.String("device", device.Name),
			zap.String("sensorType", device.Spec.SensorType))
		schedule.Status.Message = fmt.Sprintf("Device %s is not a valve (type: %s)", device.Name, device.Spec.SensorType)
		if statusErr := r.Status().Update(ctx, schedule); statusErr != nil {
			log.Error("Failed to update status", zap.Error(statusErr))
		}
		return nil, ctrl.Result{RequeueAfter: 5 * time.Minute}, fmt.Errorf("device is not a valve")
	}

	return device, ctrl.Result{}, nil
}

// updateDeviceStatus updates the resolved device name and power state
func (r *ScheduleReconciler) updateDeviceStatus(schedule *mqttv1alpha1.Schedule, device *mqttv1alpha1.Device, log *zap.Logger) {
	if schedule.Status.ResolvedDeviceName != device.Name {
		schedule.Status.ResolvedDeviceName = device.Name
	}

	if device.Status.LastPowerState != nil {
		powerState := *device.Status.LastPowerState
		if schedule.Status.ValvePowerState == nil || *schedule.Status.ValvePowerState != powerState {
			schedule.Status.ValvePowerState = &powerState
			log.Debug("Updated valve power state from device status",
				zap.Int("powerState", powerState))
		}
	}
}

// parseCronScheduleEarly parses the cron expression early to calculate next run time
// This is called before validation so we can show next run time even for disabled/invalid schedules
func (r *ScheduleReconciler) parseCronScheduleEarly(schedule *mqttv1alpha1.Schedule, log *zap.Logger) (cron.Schedule, *time.Location, time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronSchedule, err := parser.Parse(schedule.Spec.CronExpression)
	if err != nil {
		log.Error("Failed to parse cron expression",
			zap.Error(err),
			zap.String("cronExpression", schedule.Spec.CronExpression))
		return nil, nil, time.Time{}, err
	}

	loc, err := time.LoadLocation(schedule.Spec.TimeZone)
	if err != nil {
		log.Warn("Failed to load timezone, using UTC",
			zap.Error(err),
			zap.String("timeZone", schedule.Spec.TimeZone))
		loc = time.UTC
	}

	now := time.Now().In(loc)
	nextRun := cronSchedule.Next(now)

	return cronSchedule, loc, nextRun, nil
}

// shouldExecuteNow determines if the schedule should execute now
func (r *ScheduleReconciler) shouldExecuteNow(schedule *mqttv1alpha1.Schedule, cronSchedule cron.Schedule, loc *time.Location) bool {
	now := time.Now().In(loc)

	if schedule.Status.LastScheduledTime == nil {
		// First time - check if we're past the first scheduled time
		firstRun := cronSchedule.Next(schedule.CreationTimestamp.In(loc))
		return now.After(firstRun) && now.Before(firstRun.Add(1*time.Minute))
	}

	lastScheduled := schedule.Status.LastScheduledTime.In(loc)
	expectedNextRun := cronSchedule.Next(lastScheduled)

	// Execute if we're within 1 minute of the scheduled time and haven't executed yet
	if now.After(expectedNextRun.Add(-30*time.Second)) && now.Before(expectedNextRun.Add(30*time.Second)) {
		// Check if we already executed for this scheduled time
		if schedule.Status.LastExecutionTime == nil || schedule.Status.LastExecutionTime.Time.Before(lastScheduled) {
			return true
		}
	}

	return false
}

// handleIrrigationExecution starts irrigation if needed
func (r *ScheduleReconciler) handleIrrigationExecution(ctx context.Context, schedule *mqttv1alpha1.Schedule, device *mqttv1alpha1.Device, shouldExecute bool, loc *time.Location, log *zap.Logger) error {
	if !shouldExecute || schedule.Status.Active {
		return nil
	}

	if schedule.Spec.DryRun {
		log.Info("DRY-RUN: Would start scheduled irrigation",
			zap.String("device", device.Name),
			zap.Int32("durationSeconds", schedule.Spec.DurationSeconds),
			zap.String("ieeeAddr", device.Spec.IEEEAddr))
	} else {
		log.Info("Starting scheduled irrigation",
			zap.String("device", device.Name),
			zap.Int32("durationSeconds", schedule.Spec.DurationSeconds))
	}

	if err := r.executeIrrigation(ctx, schedule, device); err != nil {
		log.Error("Failed to execute irrigation", zap.Error(err))
		schedule.Status.Message = fmt.Sprintf("Failed to start irrigation: %s", err.Error())
		schedule.Status.LastStatus = "Failed"
		return err
	}

	now := metav1.Now()
	schedule.Status.Active = true
	schedule.Status.LastScheduledTime = &now
	schedule.Status.LastExecutionTime = &now
	// Format the execution time in the configured timezone
	schedule.Status.LastExecutionTimeFormatted = now.Time.In(loc).Format("2006-01-02 15:04:05 MST")
	if schedule.Spec.DryRun {
		schedule.Status.Message = "DRY-RUN: Irrigation simulation running"
		schedule.Status.LastStatus = "DryRun"
	} else {
		schedule.Status.Message = "Irrigation running"
		schedule.Status.LastStatus = "Running"
	}

	return nil
}

// handleIrrigationStop stops irrigation if duration elapsed
func (r *ScheduleReconciler) handleIrrigationStop(ctx context.Context, schedule *mqttv1alpha1.Schedule, device *mqttv1alpha1.Device, _ *time.Location, log *zap.Logger) error {
	if !schedule.Status.Active || schedule.Status.LastExecutionTime == nil {
		return nil
	}

	elapsed := time.Since(schedule.Status.LastExecutionTime.Time)
	targetDuration := time.Duration(schedule.Spec.DurationSeconds) * time.Second

	if elapsed < targetDuration {
		return nil
	}

	if schedule.Spec.DryRun {
		log.Info("DRY-RUN: Would stop irrigation",
			zap.String("device", device.Name),
			zap.Duration("elapsed", elapsed))
	} else {
		log.Info("Stopping irrigation",
			zap.String("device", device.Name),
			zap.Duration("elapsed", elapsed))
	}

	if err := r.stopIrrigation(ctx, schedule, device); err != nil {
		log.Error("Failed to stop irrigation", zap.Error(err))
		schedule.Status.Message = fmt.Sprintf("Failed to stop irrigation: %s", err.Error())
		return err
	}

	now := metav1.Now()
	schedule.Status.Active = false
	schedule.Status.LastCompletionTime = &now
	if schedule.Spec.DryRun {
		schedule.Status.Message = "DRY-RUN: Irrigation simulation completed"
		schedule.Status.LastStatus = "DryRunCompleted"
	} else {
		schedule.Status.Message = "Irrigation completed successfully"
		schedule.Status.LastStatus = "Completed"
	}

	return nil
}

// determineRequeueInterval determines when to requeue based on schedule state
func (r *ScheduleReconciler) determineRequeueInterval(schedule *mqttv1alpha1.Schedule) ctrl.Result {
	if schedule.Status.Active {
		// Check every 10 seconds while irrigation is running
		return ctrl.Result{RequeueAfter: 10 * time.Second}
	}
	// Otherwise, check every 30 seconds to catch the next scheduled time
	return ctrl.Result{RequeueAfter: 30 * time.Second}
}

// findDevice finds a device by one of the supported identifiers
func (r *ScheduleReconciler) findDevice(ctx context.Context, schedule *mqttv1alpha1.Schedule) (*mqttv1alpha1.Device, error) {
	// Count how many identifiers are specified
	identifierCount := 0
	if schedule.Spec.DeviceName != "" {
		identifierCount++
	}
	if schedule.Spec.DeviceFriendlyName != "" {
		identifierCount++
	}
	if schedule.Spec.DeviceIEEEAddr != "" {
		identifierCount++
	}
	if schedule.Spec.DeviceShortAddr != "" {
		identifierCount++
	}

	if identifierCount == 0 {
		return nil, fmt.Errorf("no device identifier specified (name, friendlyName, ieeeAddr, or shortAddr required)")
	}

	if identifierCount > 1 {
		return nil, fmt.Errorf("multiple device identifiers specified, only one should be used")
	}

	// Try to find by device name (CR name)
	if schedule.Spec.DeviceName != "" {
		device := &mqttv1alpha1.Device{}
		if err := r.Get(ctx, client.ObjectKey{
			Namespace: schedule.Namespace,
			Name:      schedule.Spec.DeviceName,
		}, device); err != nil {
			return nil, fmt.Errorf("device not found by name %s: %w", schedule.Spec.DeviceName, err)
		}
		return device, nil
	}

	// For other identifiers, we need to list all devices and search
	deviceList := &mqttv1alpha1.DeviceList{}
	if err := r.List(ctx, deviceList, client.InNamespace(schedule.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	// Search by friendly name
	if schedule.Spec.DeviceFriendlyName != "" {
		for i := range deviceList.Items {
			if deviceList.Items[i].Spec.FriendlyName == schedule.Spec.DeviceFriendlyName {
				return &deviceList.Items[i], nil
			}
		}
		return nil, fmt.Errorf("no device found with friendlyName: %s", schedule.Spec.DeviceFriendlyName)
	}

	// Search by IEEE address
	if schedule.Spec.DeviceIEEEAddr != "" {
		for i := range deviceList.Items {
			if deviceList.Items[i].Spec.IEEEAddr == schedule.Spec.DeviceIEEEAddr {
				return &deviceList.Items[i], nil
			}
		}
		return nil, fmt.Errorf("no device found with ieeeAddr: %s", schedule.Spec.DeviceIEEEAddr)
	}

	// Search by short address
	if schedule.Spec.DeviceShortAddr != "" {
		for i := range deviceList.Items {
			if deviceList.Items[i].Status.ShortAddr == schedule.Spec.DeviceShortAddr {
				return &deviceList.Items[i], nil
			}
		}
		return nil, fmt.Errorf("no device found with shortAddr: %s", schedule.Spec.DeviceShortAddr)
	}

	return nil, fmt.Errorf("no device identifier provided")
}

// executeIrrigation starts the irrigation by turning on the valve
func (r *ScheduleReconciler) executeIrrigation(ctx context.Context, _ *mqttv1alpha1.Schedule, device *mqttv1alpha1.Device) error {
	// Use the valve controller to turn on the valve
	return r.ValveController.TurnOn(ctx, device)
}

// stopIrrigation stops the irrigation by turning off the valve
func (r *ScheduleReconciler) stopIrrigation(ctx context.Context, _ *mqttv1alpha1.Schedule, device *mqttv1alpha1.Device) error {
	// Use the valve controller to turn off the valve
	return r.ValveController.TurnOff(ctx, device)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mqttv1alpha1.Schedule{}).
		Named("schedule").
		Complete(r)
}
