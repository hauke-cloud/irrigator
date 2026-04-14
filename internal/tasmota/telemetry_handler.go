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
	"encoding/json"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mqttv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
)

// TelemetryHandler processes Tasmota telemetry messages
type TelemetryHandler struct {
	client client.Client
	log    *zap.Logger
}

// NewTelemetryHandler creates a new telemetry handler
func NewTelemetryHandler(c client.Client, log *zap.Logger) *TelemetryHandler {
	return &TelemetryHandler{
		client: c,
		log:    log,
	}
}

// HandleMessage processes a telemetry message
func (h *TelemetryHandler) HandleMessage(ctx context.Context, msgCtx *MessageContext, payload []byte) error {
	var msg TelemetryMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		h.log.Error("Failed to parse telemetry message",
			zap.String("topic", msgCtx.Topic),
			zap.Error(err))
		return err
	}

	h.log.Debug("Processing telemetry message",
		zap.String("bridge", msgCtx.BridgeName),
		zap.Int("devices", len(msg.ZbReceived)))

	// Process each Zigbee device in the ZbReceived payload
	// The key in ZbReceived is the short address (e.g., "0x4F2E")
	for shortAddr, device := range msg.ZbReceived {
		if err := h.processZigbeeDevice(ctx, msgCtx, shortAddr, &device); err != nil {
			h.log.Error("Failed to process Zigbee device",
				zap.String("short_addr", shortAddr),
				zap.Error(err))
			// Continue processing other devices
			continue
		}
	}

	return nil
}

// processZigbeeDevice updates an existing Device CR for a Zigbee device
// Devices should only be created by discovery messages, not telemetry
func (h *TelemetryHandler) processZigbeeDevice(ctx context.Context, msgCtx *MessageContext, shortAddr string, device *ZigbeeDevice) error {
	// Find device by short address in status
	// List all devices and find the one with matching shortAddr
	deviceList := &mqttv1alpha1.DeviceList{}
	if err := h.client.List(ctx, deviceList, client.InNamespace(msgCtx.BridgeNamespace)); err != nil {
		return err
	}

	var existingDevice *mqttv1alpha1.Device
	for i := range deviceList.Items {
		if deviceList.Items[i].Status.ShortAddr == shortAddr {
			existingDevice = &deviceList.Items[i]
			break
		}
	}

	if existingDevice == nil {
		// Device not found - this is normal, it needs to be discovered first
		h.log.Debug("Device not found for telemetry update, skipping (device needs to be discovered first)",
			zap.String("shortAddr", shortAddr),
			zap.String("name", device.Name),
			zap.String("bridge", msgCtx.BridgeName))
		return nil
	}

	// Device exists, update status only
	return h.updateDevice(ctx, existingDevice, device)
}

// updateDevice updates an existing Device CR with telemetry data
func (h *TelemetryHandler) updateDevice(ctx context.Context, device *mqttv1alpha1.Device, zbDevice *ZigbeeDevice) error {
	// Update status
	h.updateDeviceStatus(device, zbDevice)

	if err := h.client.Status().Update(ctx, device); err != nil {
		return err
	}

	h.log.Debug("Updated Device CR",
		zap.String("device", device.Name),
		zap.String("ieee_addr", device.Spec.IEEEAddr))

	return nil
}

// updateDeviceStatus updates the device status from Zigbee device data
func (h *TelemetryHandler) updateDeviceStatus(device *mqttv1alpha1.Device, zbDevice *ZigbeeDevice) {
	now := metav1.Now()
	device.Status.LastSeen = &now
	device.Status.Available = true

	// Update link quality
	if zbDevice.LinkQuality != nil {
		lq := int32(*zbDevice.LinkQuality)
		device.Status.LinkQuality = &lq
	}

	// Update battery level
	if zbDevice.BatteryPercentage != nil {
		battery := int32(*zbDevice.BatteryPercentage)
		device.Status.BatteryLevel = &battery
	}

	// Build measurement data
	measurements := make(map[string]any)

	// Power state for valves
	if zbDevice.Power != nil {
		measurements["power"] = *zbDevice.Power
	}
	if zbDevice.Temperature != nil {
		measurements["temperature"] = *zbDevice.Temperature
	}
	if zbDevice.Humidity != nil {
		measurements["humidity"] = *zbDevice.Humidity
	}
	if zbDevice.Pressure != nil {
		measurements["pressure"] = *zbDevice.Pressure
	}
	if zbDevice.Voltage != nil {
		measurements["voltage"] = *zbDevice.Voltage
	}
	if zbDevice.Power != nil {
		measurements["power"] = *zbDevice.Power
	}
	if zbDevice.Contact != nil {
		measurements["contact"] = *zbDevice.Contact
	}
	if zbDevice.Occupancy != nil {
		measurements["occupancy"] = *zbDevice.Occupancy
	}
	if zbDevice.WaterLeak != nil {
		measurements["water_leak"] = *zbDevice.WaterLeak
	}
	if zbDevice.LinkQuality != nil {
		measurements["link_quality"] = *zbDevice.LinkQuality
	}
	if zbDevice.Endpoint != nil {
		measurements["endpoint"] = *zbDevice.Endpoint
	}

	// Convert to JSON string
	if len(measurements) > 0 {
		if jsonData, err := json.Marshal(measurements); err == nil {
			device.Status.LastMeasurement = string(jsonData)
		}
	}

	// Extract capabilities from available measurements
	capabilities := []string{}
	if zbDevice.Power != nil {
		capabilities = append(capabilities, "power")
	}
	if zbDevice.Temperature != nil {
		capabilities = append(capabilities, "temperature")
	}
	if zbDevice.Humidity != nil {
		capabilities = append(capabilities, "humidity")
	}
	if zbDevice.Pressure != nil {
		capabilities = append(capabilities, "pressure")
	}
	if zbDevice.Contact != nil {
		capabilities = append(capabilities, "contact")
	}
	if zbDevice.Occupancy != nil {
		capabilities = append(capabilities, "occupancy")
	}
	if zbDevice.WaterLeak != nil {
		capabilities = append(capabilities, "water_leak")
	}
	if zbDevice.Power != nil {
		capabilities = append(capabilities, "power")
	}

	device.Status.Capabilities = capabilities
}
