# Tasmota Controllers

This package contains handlers for processing Tasmota MQTT messages.

## Architecture

The Tasmota package uses a **dispatcher pattern** where different message types are routed to specialized handlers.

```
MQTT Message
    ↓
Dispatcher
    ↓ (routes by type)
    ├─→ TelemetryHandler (type: "telemetry")
    ├─→ StatusHandler (type: "status")
    ├─→ StateHandler (type: "state")
    └─→ InfoHandler (type: "info")
```

## Handlers

### TelemetryHandler

**File:** `telemetry_handler.go`

**Purpose:** Process Zigbee device sensor data from Tasmota bridges

**Input Message Type:** `telemetry`

**Topics:** `tele/<bridge>/SENSOR`

**Responsibilities:**
- Parse `ZbReceived` payload
- Create Device CRs for new Zigbee devices
- Update existing Device CR status
- Extract measurements (temperature, humidity, etc.)
- Update battery level and link quality

**Example Payload:**
```json
{
  "Time": "2026-04-12T21:00:00",
  "ZbReceived": {
    "0x4F2E": {
      "Device": "0x4F2E",
      "Name": "water_valve_right",
      "Power": 0,
      "LinkQuality": 61,
      "BatteryPercentage": 95,
      "Temperature": 21.5
    }
  }
}
```

### StatusHandler

**File:** `status_handler.go`

**Purpose:** Handle command responses and status updates

**Input Message Type:** `status` or `result`

**Topics:** `stat/<bridge>/RESULT`

**Responsibilities:**
- Process ZbSend command results
- Process ZbName command results
- Log command success/failure

**Example Payload:**
```json
{
  "ZbSend": {
    "Device": "0x4F2E",
    "Status": "OK"
  }
}
```

### StateHandler

**File:** `state_handler.go`

**Purpose:** Monitor Tasmota bridge health and connectivity

**Input Message Type:** `state`

**Topics:** `tele/<bridge>/STATE`

**Responsibilities:**
- Update MQTTBridge CR status
- Track connection state
- Monitor WiFi signal strength
- Log uptime

**Example Payload:**
```json
{
  "Time": "2026-04-12T21:00:00",
  "Uptime": "5T12:34:56",
  "Wifi": {
    "RSSI": 100,
    "Signal": -45
  }
}
```

### InfoHandler

**File:** `info_handler.go`

**Purpose:** Process Tasmota bridge information

**Input Message Type:** `info`

**Topics:** `tele/<bridge>/INFO1`, `tele/<bridge>/INFO2`, `tele/<bridge>/INFO3`

**Responsibilities:**
- Log Tasmota version
- Log module type
- Store bridge metadata (future)

**Example Payload:**
```json
{
  "Version": "13.2.0",
  "Module": "ESP32-C3"
}
```

## Usage

### In MQTT Manager

```go
import "github.com/hauke-cloud/irrigator/internal/tasmota"

// Create dispatcher
dispatcher := tasmota.NewDispatcher(client, zapLog)

// Dispatch a message
err := dispatcher.Dispatch(
    ctx,
    "telemetry",              // Message type from topic subscription
    "tele/tasmota_br_1/SENSOR", // MQTT topic
    "tasmota-bridge",         // Bridge name
    "default",                // Bridge namespace
    payload,                  // Message payload
)
```

### In MQTTBridge Controller

```go
// When processing a message from a topic subscription
for _, topicSub := range bridge.Spec.Topics {
    if matchesTopic(mqttTopic, topicSub.Topic) {
        if bridge.Spec.DeviceType == "tasmota" {
            dispatcher.Dispatch(
                ctx,
                topicSub.Type,     // Use the type from topic subscription
                mqttTopic,
                bridge.Name,
                bridge.Namespace,
                payload,
            )
        }
    }
}
```

## Types

All message types are defined in `types.go`:

- `TelemetryMessage` - Telemetry payload structure
- `ZigbeeDevice` - Zigbee device data within telemetry
- `StatusMessage` - Status/result payload structure
- `StateMessage` - State payload structure
- `InfoMessage` - Info payload structure
- `MessageContext` - Context for message processing

## Device CR Creation

The TelemetryHandler automatically creates Device CRs:

1. **Device Name:** Sanitized from IEEE address or Name field
2. **IEEEAddr:** Set from ZbReceived key (e.g., "0x4F2E")
3. **BridgeRef:** Points to parent MQTTBridge
4. **Labels:** Added for filtering
5. **Status:** Populated from Zigbee device data

## Extending

To add a custom handler:

```go
// Create custom handler
type CustomHandler struct {
    client client.Client
    log    *zap.Logger
}

func (h *CustomHandler) HandleMessage(ctx context.Context, msgCtx *MessageContext, payload []byte) error {
    // Your logic here
    return nil
}

// Register with dispatcher
dispatcher.RegisterHandler("custom-type", &CustomHandler{
    client: client,
    log: log,
})
```

Then use in MQTTBridge:

```yaml
spec:
  topics:
    - topic: "custom/topic/+"
      type: "custom-type"
```

## Testing

Unit tests should mock the Kubernetes client:

```go
func TestTelemetryHandler(t *testing.T) {
    // Create fake client
    scheme := runtime.NewScheme()
    _ = mqttv1alpha1.AddToScheme(scheme)
    client := fake.NewClientBuilder().WithScheme(scheme).Build()
    
    // Create handler
    handler := NewTelemetryHandler(client, zap.NewNop())
    
    // Test with sample payload
    payload := []byte(`{"ZbReceived":{"0x4F2E":{"Device":"0x4F2E"}}}`)
    ctx := &MessageContext{...}
    
    err := handler.HandleMessage(context.Background(), ctx, payload)
    assert.NoError(t, err)
}
```

## Future Improvements

- [ ] Bulk processing for multiple devices
- [ ] Retry logic for failed Device CR creation
- [ ] Metrics collection per handler
- [ ] Rate limiting
- [ ] Message validation
- [ ] Schema validation
- [ ] Device type detection
- [ ] Automatic capability discovery
