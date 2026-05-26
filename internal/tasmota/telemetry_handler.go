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
)

// TelemetryHandler processes incoming Tasmota ZbReceived telemetry messages
// and resolves any pending valve-state confirmations in the ConfirmationRegistry.
//
// To receive these messages the MQTTBridge CR must subscribe to the Tasmota
// sensor topic (tele/<tasmotaBridgeName>/SENSOR) with type "telemetry".
type TelemetryHandler struct {
	log      *zap.Logger
	registry *ConfirmationRegistry
}

// NewTelemetryHandler creates a TelemetryHandler backed by the given registry.
func NewTelemetryHandler(log *zap.Logger, registry *ConfirmationRegistry) *TelemetryHandler {
	return &TelemetryHandler{
		log:      log.With(zap.String("handler", "telemetry")),
		registry: registry,
	}
}

// HandleMessage implements MessageHandler. It parses a TelemetryMessage and
// notifies the ConfirmationRegistry for every device whose Power field is
// present in ZbReceived.
func (h *TelemetryHandler) HandleMessage(_ context.Context, msgCtx *MessageContext, payload []byte) error {
	var msg TelemetryMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		h.log.Debug("Failed to parse telemetry payload", zap.Error(err), zap.String("topic", msgCtx.Topic))
		return nil
	}

	for key, dev := range msg.ZbReceived {
		if dev.Power == nil {
			continue
		}
		if h.registry.Notify(key, *dev.Power) {
			h.log.Debug("Resolved valve confirmation from telemetry",
				zap.String("device", key),
				zap.Int("power", *dev.Power),
				zap.String("topic", msgCtx.Topic))
		}
	}

	return nil
}
