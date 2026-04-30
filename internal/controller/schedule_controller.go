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
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hauke-cloud/irrigator/internal/tasmota"
	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

// ScheduleReconciler reconciles a Schedule object
type ScheduleReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	Log                *zap.Logger
	ValveController    *tasmota.ValveController
	ConditionEvaluator *ConditionEvaluator

	// Global scheduler instance
	scheduler gocron.Scheduler
	// Map of schedule UID to job UUID
	jobs map[string]uuid.UUID
	// Map of schedule UID to cron expression + timezone (to detect changes)
	jobSpecs map[string]string
	// Map of schedule UID to last observed generation
	generations map[string]int64
	// Mutex to protect maps
	mu sync.Mutex
}

// +kubebuilder:rbac:groups=iot.hauke.cloud,resources=schedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iot.hauke.cloud,resources=schedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=iot.hauke.cloud,resources=schedules/finalizers,verbs=update
// +kubebuilder:rbac:groups=iot.hauke.cloud,resources=devices,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *ScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.With(
		zap.String("namespace", req.Namespace),
		zap.String("name", req.Name))

	log.Debug("Reconciling Schedule")

	// Fetch the Schedule instance
	schedule := &iotv1alpha1.Schedule{}
	err := r.Get(ctx, req.NamespacedName, schedule)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debug("Schedule deleted, removing cron job")
			r.removeJob(string(schedule.UID))
			return ctrl.Result{}, nil
		}
		log.Error("Failed to get Schedule", zap.Error(err))
		return ctrl.Result{}, err
	}

	// Check if this is just a status update (generation hasn't changed)
	scheduleUID := string(schedule.UID)
	r.mu.Lock()
	lastGeneration, exists := r.generations[scheduleUID]
	currentGeneration := schedule.Generation
	r.mu.Unlock()

	if exists && lastGeneration == currentGeneration {
		// Spec hasn't changed, just a status update - skip reconciliation
		log.Debug("Skipping reconcile - only status changed", zap.Int64("generation", currentGeneration))
		return ctrl.Result{}, nil
	}

	// Load timezone
	loc, err := time.LoadLocation(schedule.Spec.TimeZone)
	if err != nil {
		log.Warn("Invalid timezone, using UTC", zap.Error(err))
		loc = time.UTC
	}

	// Check if schedule is enabled
	if schedule.Spec.Enabled != nil && !*schedule.Spec.Enabled {
		r.removeJob(scheduleUID)
		schedule.Status.Message = "Schedule is disabled"
		schedule.Status.Active = false
		if err := r.updateStatus(ctx, schedule, log); err != nil {
			return ctrl.Result{}, err
		}
		r.updateGeneration(scheduleUID, currentGeneration)
		return ctrl.Result{}, nil
	}

	// Find and validate the device
	device, err := r.findDevice(ctx, schedule)
	if err != nil {
		r.removeJob(scheduleUID)
		schedule.Status.Message = fmt.Sprintf("Device not found: %s", err.Error())
		if statusErr := r.updateStatus(ctx, schedule, log); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		r.updateGeneration(scheduleUID, currentGeneration)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	// Verify device is a valve
	if device.Spec.SensorType != "valve" {
		r.removeJob(scheduleUID)
		schedule.Status.Message = fmt.Sprintf("Device %s is not a valve", device.Name)
		if statusErr := r.updateStatus(ctx, schedule, log); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		r.updateGeneration(scheduleUID, currentGeneration)
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, fmt.Errorf("device is not a valve")
	}

	// Update device status
	schedule.Status.ResolvedDeviceName = device.Name
	// Note: ValvePowerState removed - power state is now tracked by mqtt-sensor-exporter

	// Create or update the cron job
	if err := r.ensureJob(ctx, schedule, device, loc, log); err != nil {
		schedule.Status.Message = fmt.Sprintf("Failed to setup schedule: %s", err.Error())
		if statusErr := r.updateStatus(ctx, schedule, log); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		r.updateGeneration(scheduleUID, currentGeneration)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	// Update status with next run time and message
	if err := r.updateNextRunTime(schedule, loc); err != nil {
		log.Warn("Failed to get next run time", zap.Error(err))
		schedule.Status.Message = fmt.Sprintf("Schedule active but unable to determine next run: %s", err.Error())
	} else {
		// Set success message when not actively irrigating
		if !schedule.Status.Active {
			schedule.Status.Message = "Schedule is active and waiting for next run"
		}
	}

	// Update status
	if err := r.updateStatus(ctx, schedule, log); err != nil {
		return ctrl.Result{}, err
	}

	// Remember this generation
	r.updateGeneration(scheduleUID, currentGeneration)

	return ctrl.Result{}, nil
}

// updateStatus updates the status subresource, handling conflicts gracefully
func (r *ScheduleReconciler) updateStatus(ctx context.Context, schedule *iotv1alpha1.Schedule, log *zap.Logger) error {
	if err := r.Status().Update(ctx, schedule); err != nil {
		// Conflicts are expected when status is updated from multiple places (e.g., from cron jobs)
		// The controller will retry automatically
		if errors.IsConflict(err) {
			log.Debug("Status update conflict, will retry", zap.Error(err))
			return nil
		}
		log.Error("Failed to update status", zap.Error(err))
		return err
	}
	return nil
}

// updateGeneration stores the last observed generation
func (r *ScheduleReconciler) updateGeneration(scheduleUID string, generation int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generations[scheduleUID] = generation
}

// ensureJob creates or updates a cron job for the schedule
func (r *ScheduleReconciler) ensureJob(ctx context.Context, schedule *iotv1alpha1.Schedule, device *iotv1alpha1.Device, loc *time.Location, log *zap.Logger) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	scheduleUID := string(schedule.UID)
	currentSpec := fmt.Sprintf("%s|%s", schedule.Spec.CronExpression, schedule.Spec.TimeZone)

	// Check if job already exists and if spec has changed
	if jobID, exists := r.jobs[scheduleUID]; exists {
		// Check if cron expression or timezone changed
		if lastSpec, specExists := r.jobSpecs[scheduleUID]; specExists && lastSpec == currentSpec {
			// Verify the job still exists in the scheduler
			for _, j := range r.scheduler.Jobs() {
				if j.ID() == jobID {
					log.Debug("Cron job already exists with same spec, skipping creation", zap.String("jobID", jobID.String()))
					return nil
				}
			}
			// Job was removed from scheduler, clean up our map
			log.Debug("Job ID in map but not in scheduler, will recreate")
		} else {
			log.Info("Cron expression or timezone changed, recreating job",
				zap.String("oldSpec", lastSpec),
				zap.String("newSpec", currentSpec))
		}

		// Remove the old job
		if err := r.scheduler.RemoveJob(jobID); err != nil {
			log.Warn("Failed to remove old job", zap.Error(err))
		}
		delete(r.jobs, scheduleUID)
		delete(r.jobSpecs, scheduleUID)
	}

	// Create the irrigation task
	task := func() {
		r.executeScheduledIrrigation(ctx, schedule, device, loc, log)
	}

	// Create cron expression with timezone prefix
	cronExprWithTZ := fmt.Sprintf("TZ=%s %s", schedule.Spec.TimeZone, schedule.Spec.CronExpression)

	// Create a new cron job with the schedule's timezone
	job, err := r.scheduler.NewJob(
		gocron.CronJob(cronExprWithTZ, false),
		gocron.NewTask(task),
		gocron.WithName(fmt.Sprintf("%s/%s", schedule.Namespace, schedule.Name)),
	)
	if err != nil {
		return fmt.Errorf("failed to create cron job: %w", err)
	}

	// Store the job UUID and spec
	r.jobs[scheduleUID] = job.ID()
	r.jobSpecs[scheduleUID] = currentSpec

	log.Info("Created cron job for schedule",
		zap.String("cronExpression", schedule.Spec.CronExpression),
		zap.String("timezone", schedule.Spec.TimeZone),
		zap.String("jobID", job.ID().String()))

	return nil
}

// removeJob removes a cron job for the schedule
func (r *ScheduleReconciler) removeJob(scheduleUID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if jobID, exists := r.jobs[scheduleUID]; exists {
		if err := r.scheduler.RemoveJob(jobID); err != nil {
			r.Log.Warn("Failed to remove job", zap.Error(err), zap.String("scheduleUID", scheduleUID))
		}
		delete(r.jobs, scheduleUID)
	}
	// Also remove spec tracking and generation tracking
	delete(r.jobSpecs, scheduleUID)
	delete(r.generations, scheduleUID)
}

// updateNextRunTime updates the status with next run time from the cron job
func (r *ScheduleReconciler) updateNextRunTime(schedule *iotv1alpha1.Schedule, loc *time.Location) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	scheduleUID := string(schedule.UID)
	jobID, exists := r.jobs[scheduleUID]
	if !exists {
		return fmt.Errorf("job not found")
	}

	// Find the job by ID
	var job gocron.Job
	for _, j := range r.scheduler.Jobs() {
		if j.ID() == jobID {
			job = j
			break
		}
	}

	if job == nil {
		return fmt.Errorf("job not found in scheduler")
	}

	nextRun, err := job.NextRun()
	if err != nil {
		return fmt.Errorf("failed to get next run: %w", err)
	}

	nextRunTime := metav1.NewTime(nextRun)
	schedule.Status.NextScheduledTime = &nextRunTime
	schedule.Status.NextScheduledTimeFormatted = nextRun.In(loc).Format("2006-01-02 15:04:05 MST")

	// Update last execution time formatted if we have a last execution time
	if schedule.Status.LastExecutionTime != nil {
		schedule.Status.LastExecutionTimeFormatted = schedule.Status.LastExecutionTime.Time.In(loc).Format("2006-01-02 15:04:05 MST")
	}

	return nil
}

// executeScheduledIrrigation is called by the cron job to run irrigation
func (r *ScheduleReconciler) executeScheduledIrrigation(ctx context.Context, schedule *iotv1alpha1.Schedule, device *iotv1alpha1.Device, loc *time.Location, log *zap.Logger) {
	log.Info("Executing scheduled irrigation",
		zap.String("schedule", schedule.Name),
		zap.String("device", device.Name),
		zap.Int32("durationSeconds", schedule.Spec.DurationSeconds),
		zap.Bool("dryRun", schedule.Spec.DryRun))

	// Update valve controller dry-run mode
	r.ValveController.SetDryRun(schedule.Spec.DryRun)

	// Fetch the latest schedule to ensure we have fresh status
	if err := r.Get(ctx, client.ObjectKey{Namespace: schedule.Namespace, Name: schedule.Name}, schedule); err != nil {
		log.Error("Failed to fetch schedule for execution", zap.Error(err))
		return
	}

	// Evaluate execution conditions
	now := metav1.Now()
	schedule.Status.ConditionsLastChecked = &now

	if len(schedule.Spec.ExecutionConditions) > 0 {
		passed, message, err := r.ConditionEvaluator.EvaluateConditions(ctx, schedule)
		schedule.Status.ConditionsPassed = &passed
		schedule.Status.ConditionsMessage = message

		if err != nil {
			log.Error("Failed to evaluate conditions", zap.Error(err))
			schedule.Status.Message = fmt.Sprintf("Condition evaluation error: %s", err.Error())
			schedule.Status.LastStatus = "ConditionError"
			_ = r.updateStatus(ctx, schedule, log)
			return
		}

		if !passed {
			log.Info("Execution conditions not met, skipping irrigation",
				zap.String("schedule", schedule.Name),
				zap.String("message", message))
			schedule.Status.Message = fmt.Sprintf("Skipped: %s", message)
			schedule.Status.LastStatus = "ConditionsNotMet"
			schedule.Status.LastScheduledTime = &now
			_ = r.updateStatus(ctx, schedule, log)
			return
		}

		log.Info("Execution conditions met, proceeding with irrigation",
			zap.String("schedule", schedule.Name))
	}

	// Start irrigation
	schedule.Status.Active = true
	schedule.Status.LastScheduledTime = &now
	schedule.Status.LastExecutionTime = &now
	schedule.Status.LastExecutionTimeFormatted = now.Time.In(loc).Format("2006-01-02 15:04:05 MST")

	// Update next scheduled time for the next run
	scheduleUID := string(schedule.UID)
	r.mu.Lock()
	jobID, exists := r.jobs[scheduleUID]
	r.mu.Unlock()

	if exists {
		for _, j := range r.scheduler.Jobs() {
			if j.ID() == jobID {
				if nextRun, err := j.NextRun(); err == nil {
					nextRunTime := metav1.NewTime(nextRun)
					schedule.Status.NextScheduledTime = &nextRunTime
					schedule.Status.NextScheduledTimeFormatted = nextRun.In(loc).Format("2006-01-02 15:04:05 MST")
				}
				break
			}
		}
	}

	if schedule.Spec.DryRun {
		schedule.Status.Message = "DRY-RUN: Irrigation simulation running"
		schedule.Status.LastStatus = "DryRun"
		log.Info("DRY-RUN: Would start irrigation", zap.String("device", device.Name))
	} else {
		if err := r.ValveController.TurnOn(ctx, device); err != nil {
			log.Error("Failed to start irrigation", zap.Error(err))
			schedule.Status.Message = fmt.Sprintf("Failed to start irrigation: %s", err.Error())
			schedule.Status.LastStatus = "Failed"
			schedule.Status.Active = false
			_ = r.updateStatus(ctx, schedule, log)
			return
		}
		schedule.Status.Message = "Irrigation running"
		schedule.Status.LastStatus = "Running"
	}

	_ = r.updateStatus(ctx, schedule, log)

	// Schedule the stop after duration
	duration := time.Duration(schedule.Spec.DurationSeconds) * time.Second
	time.AfterFunc(duration, func() {
		r.stopScheduledIrrigation(ctx, schedule, device, log)
	})
}

// stopScheduledIrrigation stops the irrigation after duration
func (r *ScheduleReconciler) stopScheduledIrrigation(ctx context.Context, schedule *iotv1alpha1.Schedule, device *iotv1alpha1.Device, log *zap.Logger) {
	log.Info("Stopping scheduled irrigation",
		zap.String("schedule", schedule.Name),
		zap.String("device", device.Name),
		zap.Bool("dryRun", schedule.Spec.DryRun))

	// Fetch the latest schedule to ensure we have fresh status
	if err := r.Get(ctx, client.ObjectKey{Namespace: schedule.Namespace, Name: schedule.Name}, schedule); err != nil {
		log.Error("Failed to fetch schedule for stop", zap.Error(err))
		return
	}

	if schedule.Spec.DryRun {
		log.Info("DRY-RUN: Would stop irrigation", zap.String("device", device.Name))
		schedule.Status.Message = "DRY-RUN: Irrigation simulation completed"
		schedule.Status.LastStatus = "DryRunCompleted"
	} else {
		if err := r.ValveController.TurnOff(ctx, device); err != nil {
			log.Error("Failed to stop irrigation", zap.Error(err))
			schedule.Status.Message = fmt.Sprintf("Failed to stop irrigation: %s", err.Error())
			_ = r.updateStatus(ctx, schedule, log)
			return
		}
		schedule.Status.Message = "Irrigation completed successfully"
		schedule.Status.LastStatus = "Completed"
	}

	now := metav1.Now()
	schedule.Status.Active = false
	schedule.Status.LastCompletionTime = &now

	_ = r.updateStatus(ctx, schedule, log)
}

// findDevice finds a device by one of the supported identifiers
func (r *ScheduleReconciler) findDevice(ctx context.Context, schedule *iotv1alpha1.Schedule) (*iotv1alpha1.Device, error) {
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
		device := &iotv1alpha1.Device{}
		if err := r.Get(ctx, client.ObjectKey{
			Namespace: schedule.Namespace,
			Name:      schedule.Spec.DeviceName,
		}, device); err != nil {
			return nil, fmt.Errorf("device not found by name %s: %w", schedule.Spec.DeviceName, err)
		}
		return device, nil
	}

	// For other identifiers, we need to list all devices and search
	deviceList := &iotv1alpha1.DeviceList{}
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

// SetupWithManager sets up the controller with the Manager.
func (r *ScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize the global scheduler
	s, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}
	r.scheduler = s
	r.jobs = make(map[string]uuid.UUID)
	r.jobSpecs = make(map[string]string)
	r.generations = make(map[string]int64)

	// Start the scheduler
	r.scheduler.Start()

	return ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.Schedule{}).
		Named("schedule").
		Complete(r)
}

// Shutdown stops the scheduler
func (r *ScheduleReconciler) Shutdown() error {
	if r.scheduler != nil {
		return r.scheduler.Shutdown()
	}
	return nil
}
