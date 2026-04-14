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

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mqttv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
)

// BridgeConnector defines an interface for connecting to MQTT bridges
type BridgeConnector interface {
	Connect(ctx context.Context, bridge *mqttv1alpha1.MQTTBridge) error
}

// ValveController manages valve operations via Tasmota MQTT commands
type ValveController struct {
	client          client.Client
	log             *zap.Logger
	mqttPublisher   MQTTPublisher
	bridgeConnector BridgeConnector
	dryRun          bool
}

// NewValveController creates a new valve controller
func NewValveController(c client.Client, log *zap.Logger, mqttPublisher MQTTPublisher, bridgeConnector BridgeConnector, dryRun bool) *ValveController {
	return &ValveController{
		client:          c,
		log:             log.With(zap.String("component", "valve-controller")),
		mqttPublisher:   mqttPublisher,
		bridgeConnector: bridgeConnector,
		dryRun:          dryRun,
	}
}

// TurnOn turns on a valve device
func (v *ValveController) TurnOn(ctx context.Context, device *mqttv1alpha1.Device) error {
	return v.setPowerState(ctx, device, "1", "ON")
}

// TurnOff turns off a valve device
func (v *ValveController) TurnOff(ctx context.Context, device *mqttv1alpha1.Device) error {
	return v.setPowerState(ctx, device, "0", "OFF")
}

// setPowerState sets the power state of a valve
func (v *ValveController) setPowerState(ctx context.Context, device *mqttv1alpha1.Device, powerValue, state string) error {
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
	bridge := &mqttv1alpha1.MQTTBridge{}
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
		// Dry-run mode: Just log what would be done
		log.Info("DRY-RUN: Would send valve command",
			zap.String("topic", fmt.Sprintf("cmnd/%s/ZbSend", bridge.Spec.BridgeName)),
			zap.String("payload", payload))
		return nil
	}

	// Send the MQTT command
	log.Info("Sending valve command",
		zap.String("payload", payload))

	if err := v.mqttPublisher.PublishTasmotaCommand(
		bridgeNamespace,
		bridgeName,
		"ZbSend",
		payload,
	); err != nil {
		return fmt.Errorf("failed to publish valve %s command: %w", state, err)
	}

	log.Info("Valve command sent successfully")
	return nil
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
