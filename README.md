

<a href="https://hauke.cloud" target="_blank"><img src="https://img.shields.io/badge/home-hauke.cloud-brightgreen" alt="hauke.cloud" style="display: block;" /></a>
<a href="https://github.com/hauke-cloud" target="_blank"><img src="https://img.shields.io/badge/github-hauke.cloud-blue" alt="hauke.cloud Github Organisation" style="display: block;" /></a>

# Irrigator - Kubernetes Irrigation Scheduler

<img src="https://raw.githubusercontent.com/hauke-cloud/.github/main/resources/img/organisation-logo-small.png" alt="hauke.cloud logo" width="109" height="123" align="right">

A Kubernetes operator that manages automated irrigation schedules for Tasmota-connected valve devices. Built with kubebuilder, this operator provides a declarative, cloud-native way to schedule and control irrigation valves using cron expressions and MQTT.

## Features

- **Cron-based Scheduling**: Use standard cron expressions to define irrigation times
- **Timezone Support**: Configure schedules in your local timezone
- **Duration Control**: Specify how long valves should remain open (in seconds)
- **Enable/Disable**: Easily enable or disable schedules without deleting them
- **Dry-Run Mode**: Test schedules and see execution plans without actually controlling valves
- **Status Tracking**: Monitor last run, next run, and current irrigation state
- **Automatic Valve Control**: Automatically turns valves on and off via Tasmota MQTT commands
- **Power State Tracking**: Reads valve power state from device telemetry
- **Multiple Addressing Options**: Reference devices by name, friendly name, IEEE address, or short address

## Architecture

The operator watches three types of resources:
- **MQTTBridge** (read-only): Manages MQTT broker connections to Tasmota bridges
- **Device** (read-only): Represents Zigbee/MQTT devices discovered through Tasmota
- **Schedule** (managed): Manages irrigation schedules for valve devices

### Custom Resources

#### Schedule
The primary resource managed by this operator. Defines when and how long to irrigate a valve device.

**Spec Fields:**
- `deviceName` / `deviceFriendlyName` / `deviceIEEEAddr` / `deviceShortAddr`: Device identifier (one required)
- `cronExpression`: Standard cron expression (minute hour day month weekday)
- `durationSeconds`: How long to run irrigation (1-86400 seconds)
- `enabled`: Whether schedule is active (default: true)
- `timeZone`: Timezone for cron schedule (default: UTC)
- `dryRun`: Enable dry-run mode - logs execution plan without sending MQTT commands (default: false)

**Status Fields:**
- `resolvedDeviceName`: The actual Device CR name that was found
- `valvePowerState`: Last known valve power state from device telemetry
- `lastScheduledTime`: When irrigation was last scheduled
- `lastExecutionTime`: When irrigation actually started
- `lastCompletionTime`: When irrigation finished
- `nextScheduledTime`: Next scheduled run time
- `active`: Whether irrigation is currently running
- `lastStatus`: Result of last execution (Running/Completed/Failed/DryRun/DryRunCompleted)

#### MQTTBridge (Read-Only)
Represents an MQTT broker connection to Tasmota. Managed by an external device management controller.

**Key Fields:**
- Host, port, and TLS configuration
- Credentials stored in Kubernetes Secrets
- Connection status monitoring

#### Device (Read-Only)
Represents a Zigbee valve device connected through Tasmota. Managed by an external device management controller.

**Key Fields:**
- `spec.bridgeRef`: Reference to parent MQTTBridge
- `spec.ieeeAddr`: Unique IEEE address from Zigbee
- `spec.sensorType`: Must be "valve" for irrigation schedules
- `spec.friendlyName`: Human-readable name
- `status.shortAddr`: Short Zigbee address
- `status.lastPowerState`: Current valve power state (updated by telemetry)

## Getting Started

### Prerequisites

- Kubernetes cluster (v1.29+)
- kubectl configured
- MQTT broker (e.g., Mosquitto)
- Tasmota bridge with Zigbee coordinator
- Valve devices with `sensorType: valve`
- MQTTBridge and Device CRDs installed (managed by external device management controller)

### Installation

**Prerequisites:**
- MQTTBridge and Device CRDs must be installed by the external device management controller
- This operator only manages the Schedule CRD

The operator automatically installs/upgrades the Schedule CRD at startup by default.

**Development mode** (runs locally, installs Schedule CRD automatically):
```bash
make run
```

**Production deployment:**
```bash
make docker-build docker-push IMG=<your-registry>/irrigator:tag
make deploy IMG=<your-registry>/irrigator:tag
```

The operator will automatically:
- Install the Schedule CRD if it doesn't exist
- Upgrade the Schedule CRD if it already exists
- Read MQTTBridge and Device resources (managed by external controller)

**Note:** If you prefer manual CRD installation, you can disable automatic installation:
```bash
# Run with CRD auto-install disabled
./bin/manager --install-crds=false

# Or manually install the Schedule CRD
kubectl apply -f config/crd/bases/mqtt.hauke.cloud_schedules.yaml
```

### Quick Start

1. **Verify your valve device exists:**

```bash
kubectl get devices
```

Ensure you have a Device CR with `sensorType: valve`. Example:
```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: Device
metadata:
  name: garden-valve-1
spec:
  bridgeRef:
    name: my-tasmota-bridge
  ieeeAddr: "0x00158D0001234567"
  sensorType: valve
  friendlyName: "Garden Valve Zone 1"
```

2. **Create an irrigation schedule:**

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: Schedule
metadata:
  name: morning-watering
spec:
  deviceName: garden-valve-1
  cronExpression: "0 6 * * *"  # Every day at 6:00 AM
  durationSeconds: 1800         # 30 minutes
  enabled: true
  timeZone: "Europe/Berlin"
```

```bash
kubectl apply -f schedule.yaml
```

3. **Monitor the schedule:**

```bash
# List all schedules
kubectl get schedules

# Watch for status updates
kubectl get schedule morning-watering -w

# Get detailed information
kubectl describe schedule morning-watering
```

### Configuration Examples

#### Cron Expression Examples

```yaml
# Every day at 6:00 AM
cronExpression: "0 6 * * *"

# Twice daily: 6:00 AM and 6:00 PM
cronExpression: "0 6,18 * * *"

# Every other day (Mon, Wed, Fri) at 7:00 AM
cronExpression: "0 7 * * 1,3,5"

# Every 2 hours
cronExpression: "0 */2 * * *"

# Monday through Friday at 8:00 AM
cronExpression: "0 8 * * 1-5"
```

#### Device Addressing Options

You can reference devices in multiple ways:

**By Device Name** (fastest, recommended):
```yaml
spec:
  deviceName: garden-valve-1
```

**By Friendly Name**:
```yaml
spec:
  deviceFriendlyName: "Garden Valve Zone 1"
```

**By IEEE Address**:
```yaml
spec:
  deviceIEEEAddr: "0x00158D0001234567"
```

**By Short Address**:
```yaml
spec:
  deviceShortAddr: "0x1234"
```

#### Multiple Zones

Create separate schedules for different zones:

```yaml
---
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: Schedule
metadata:
  name: zone-1-morning
spec:
  deviceName: valve-zone-1
  cronExpression: "0 6 * * *"
  durationSeconds: 1800
---
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: Schedule
metadata:
  name: zone-2-morning
spec:
  deviceName: valve-zone-2
  cronExpression: "30 6 * * *"  # 30 minutes after zone 1
  durationSeconds: 1800
```

#### Dry-Run Mode

Test schedules without controlling actual valves:

```yaml
spec:
  dryRun: true
```

Watch the logs to see execution plans:
```bash
kubectl logs -n irrigator-system deployment/irrigator-controller-manager -f | grep DRY-RUN
```

## How It Works

1. **Reconciliation Loop**: The Schedule controller runs every 30 seconds to check for scheduled irrigation times
2. **Execution Detection**: When the current time matches the cron schedule (within ±30 seconds), irrigation starts
3. **Valve Control**: The controller delegates to the ValveController which sends MQTT `ZbSend` commands to Tasmota
4. **Duration Monitoring**: While irrigation is active, the controller checks every 10 seconds
5. **Automatic Shutoff**: When the specified duration elapses, the valve is automatically turned off
6. **Power State Tracking**: The controller reads the valve's power state from Device status (updated by telemetry from external controller)

### MQTT Commands

The controller sends Tasmota Zigbee commands using the IEEE address:

**Turn valve ON:**
```json
{"Device":"0xF4B3B1FFFE4EA459","Send":{"Power":"1"}}
```

**Turn valve OFF:**
```json
{"Device":"0xF4B3B1FFFE4EA459","Send":{"Power":"0"}}
```

These commands are published to: `cmnd/<bridge-name>/ZbSend`

## Operations

### Temporarily Disable Schedule
```bash
kubectl patch schedule morning-watering \
  --type=merge \
  -p '{"spec":{"enabled":false}}'
```

### Update Duration
```bash
kubectl patch schedule morning-watering \
  --type=merge \
  -p '{"spec":{"durationSeconds":3600}}'  # Change to 1 hour
```

### Change Schedule Time
```bash
kubectl patch schedule morning-watering \
  --type=merge \
  -p '{"spec":{"cronExpression":"0 7 * * *"}}'  # Change to 7:00 AM
```

## Development

### Project Structure

```
.
├── api/v1alpha1/          # CRD definitions
│   ├── mqttbridge_types.go
│   ├── device_types.go
│   └── schedule_types.go
├── internal/
│   ├── controller/        # Reconciliation logic
│   │   └── schedule_controller.go
│   ├── mqtt/             # MQTT client management
│   │   └── manager.go
│   └── tasmota/          # Tasmota valve control
│       └── valve_controller.go
├── config/               # Kubernetes manifests
│   ├── crd/             # Generated CRDs
│   ├── rbac/            # RBAC rules
│   └── samples/         # Example resources
└── cmd/                 # Main entry point
```

### Building

```bash
# Generate code and manifests
make generate manifests

# Run tests
make test

# Build binary
make build

# Build and push Docker image
make docker-build docker-push IMG=<registry>/irrigator:tag
```

### Testing

Run unit tests:
```bash
make test
```

Run with a local Kubernetes cluster (kind/minikube):
```bash
make run
```

Run linter:
```bash
make lint
```

## Safety Features

- **Automatic Shutoff**: Valves automatically turn off after `durationSeconds`
- **Manual Override**: Delete schedule to stop scheduled irrigation
- **Enable/Disable**: Quickly disable without losing configuration
- **Dry-Run Mode**: Test schedules safely without controlling valves
- **Status Tracking**: Always know when last/next irrigation will occur
- **Centralized Control**: All valve operations go through ValveController

## Troubleshooting

### Schedule Not Running

1. Check if enabled: `kubectl get schedule <name> -o jsonpath='{.spec.enabled}'`
2. Check next scheduled time: `kubectl get schedule <name> -o jsonpath='{.status.nextScheduledTime}'`
3. Check for errors: `kubectl describe schedule <name>`

### Device Not Found

Error: `Device not found: garden-valve-1`

Verify device exists: `kubectl get device garden-valve-1`

### Device Not a Valve

Error: `Device garden-valve-1 is not a valve`

Check device sensorType: `kubectl get device garden-valve-1 -o jsonpath='{.spec.sensorType}'`

It should return `valve`.

### Invalid Cron Expression

Verify cron syntax using https://crontab.guru

Format: `minute hour day month weekday`

### MQTT Connection Issues

Check operator logs:
```bash
kubectl logs -n irrigator-system deployment/irrigator-controller-manager -f
```

## Documentation

- [QUICKSTART.md](QUICKSTART.md) - Quick start guide with examples
- [IRRIGATION.md](IRRIGATION.md) - Detailed irrigation scheduler documentation
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contributing guidelines

## Contributing

To become a contributor, please check out the [CONTRIBUTING](CONTRIBUTING.md) file.

## License

This project is licensed under the Apache License, Version 2.0 - see the [LICENSE](LICENSE) file for details.

## Contact

For any inquiries or support requests, please open an issue in this repository or contact us at [contact@hauke.cloud](mailto:contact@hauke.cloud).

