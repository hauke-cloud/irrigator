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
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

const (
	confirmTimeout = 10 * time.Second
	retryDelay     = 2 * time.Second
	maxRetries     = 3
)

// BridgeConnector defines an interface for connecting to MQTT bridges
type BridgeConnector interface {
	Connect(ctx context.Context, bridge *iotv1alpha1.MQTTBridge) error
}

// ValveController manages valve operations via Tasmota MQTT commands
type ValveController struct {
	client          client.Client
	log             *zap.Logger
	mqttPublisher   MQTTPublisher
	bridgeConnector BridgeConnector
	confirmRegistry *ConfirmationRegistry
	dryRun          bool
}

// NewValveController creates a new valve controller.
// confirmRegistry may be nil, in which case commands are fire-and-forget with
// no state verification. When non-nil the controller waits for a ZbReceived
// telemetry confirmation and retries up to maxRetries times on mismatch or
// timeout.
func NewValveController(c client.Client, log *zap.Logger, mqttPublisher MQTTPublisher, bridgeConnector BridgeConnector, dryRun bool, confirmRegistry *ConfirmationRegistry) *ValveController {
	return &ValveController{
		client:          c,
		log:             log.With(zap.String("component", "valve-controller")),
		mqttPublisher:   mqttPublisher,
		bridgeConnector: bridgeConnector,
		confirmRegistry: confirmRegistry,
		dryRun:          dryRun,
	}
}

// TurnOn turns on a valve device
func (v *ValveController) TurnOn(ctx context.Context, device *iotv1alpha1.Device) error {
	return v.setPowerState(ctx, device, "1", "ON")
}

// TurnOff turns off a valve device
func (v *ValveController) TurnOff(ctx context.Context, device *iotv1alpha1.Device) error {
	return v.setPowerState(ctx, device, "0", "OFF")
}

// setPowerState sets the power state of a valve and, when a ConfirmationRegistry
// is configured, waits for a ZbReceived telemetry message to verify the state
// change. It retries up to maxRetries times on timeout or mismatch.
func (v *ValveController) setPowerState(ctx context.Context, device *iotv1alpha1.Device, powerValue, state string) error {
	// Validate device has required fields
	if device.Spec.IEEEAddr == "" {
		return fmt.Errorf("device %s has no IEEE address", device.Name)
	}

	// Get the bridge reference
	bridgeNamespace := device.Spec.BridgeRef.Namespace
	if bridgeNamespace == "" {
		bridgeNamespace = device.Namespace
	}
	bridgeName := device.Spec.BridgeRef.Name

	// Fetch the bridge to verify it exists and is Tasmota
	bridge := &iotv1alpha1.MQTTBridge{}
	if err := v.client.Get(ctx, client.ObjectKey{
		Namespace: bridgeNamespace,
		Name:      bridgeName,
	}, bridge); err != nil {
		return fmt.Errorf("failed to get bridge: %w", err)
	}

	if bridge.Spec.DeviceType != "tasmota" {
		return fmt.Errorf("bridge %s is not Tasmota type (type: %s)", bridgeName, bridge.Spec.DeviceType)
	}

	// Ensure bridge is connected (on-demand connection)
	if err := v.bridgeConnector.Connect(ctx, bridge); err != nil {
		v.log.Warn("Failed to connect to bridge, will attempt to publish anyway",
			zap.Error(err),
			zap.String("bridge", bridgeName))
		// Continue anyway - connection might already exist
	}

	// Build the MQTT payload
	// Format: {"Device":"0xF4B3B1FFFE4EA459","Send":{"Power":"0"}}
	payload := fmt.Sprintf(`{"Device":"%s","Send":{"Power":"%s"}}`, device.Spec.IEEEAddr, powerValue)

	log := v.log.With(
		zap.String("device", device.Name),
		zap.String("ieeeAddr", device.Spec.IEEEAddr),
		zap.String("bridge", bridgeName),
		zap.String("state", state),
		zap.Bool("dryRun", v.dryRun),
	)

	if v.dryRun {
		log.Info("DRY-RUN: Would send valve command",
			zap.String("topic", fmt.Sprintf("cmnd/%s/ZbSend", bridge.Spec.BridgeName)),
			zap.String("payload", payload))
		return nil
	}

	expectedPower := 0
	if powerValue == "1" {
		expectedPower = 1
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Info("Retrying valve command",
				zap.Int("attempt", attempt),
				zap.Int("maxRetries", maxRetries))
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Register confirmation before publishing to avoid a race where the
		// telemetry arrives before we start listening.
		var confirmCh <-chan ConfirmationResult
		if v.confirmRegistry != nil {
			confirmCh = v.confirmRegistry.Register(device.Spec.IEEEAddr, expectedPower)
		}

		log.Info("Sending valve command",
			zap.String("payload", payload),
			zap.Int("attempt", attempt))

		if err := v.mqttPublisher.PublishTasmotaCommand(
			bridgeNamespace,
			bridgeName,
			"ZbSend",
			payload,
		); err != nil {
			if v.confirmRegistry != nil {
				v.confirmRegistry.Deregister(device.Spec.IEEEAddr)
			}
			return fmt.Errorf("failed to publish valve %s command: %w", state, err)
		}

		// No registry configured: fire-and-forget, same as before.
		if confirmCh == nil {
			log.Info("Valve command sent successfully (no confirmation)")
			return nil
		}

		// Wait for the ZbReceived telemetry to confirm the actual device state.
		select {
		case result := <-confirmCh:
			if result.Confirmed {
				log.Info("Valve state confirmed",
					zap.String("state", state),
					zap.Int("attempt", attempt))
				return nil
			}
			log.Warn("Valve state mismatch — will retry",
				zap.Int("attempt", attempt),
				zap.Int("expected", expectedPower),
				zap.Int("actual", result.ActualPower))
		case <-time.After(confirmTimeout):
			v.confirmRegistry.Deregister(device.Spec.IEEEAddr)
			log.Warn("Valve state confirmation timed out — will retry",
				zap.Int("attempt", attempt),
				zap.Duration("timeout", confirmTimeout))
		case <-ctx.Done():
			v.confirmRegistry.Deregister(device.Spec.IEEEAddr)
			return ctx.Err()
		}
	}

	return fmt.Errorf("valve %s: failed to confirm state %q after %d attempts", device.Name, state, maxRetries)
}

// SetDryRun enables or disables dry-run mode
func (v *ValveController) SetDryRun(enabled bool) {
	v.dryRun = enabled
	v.log.Info("Dry-run mode updated", zap.Bool("enabled", enabled))
}

// IsDryRun returns whether dry-run mode is enabled
func (v *ValveController) IsDryRun() bool {
	return v.dryRun
}
