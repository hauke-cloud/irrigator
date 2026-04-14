

<a href="https://hauke.cloud" target="_blank"><img src="https://img.shields.io/badge/home-hauke.cloud-brightgreen" alt="hauke.cloud" style="display: block;" /></a>
<a href="https://github.com/hauke-cloud" target="_blank"><img src="https://img.shields.io/badge/github-hauke.cloud-blue" alt="hauke.cloud Github Organisation" style="display: block;" /></a>

# MQTT Sensor Exporter

<img src="https://raw.githubusercontent.com/hauke-cloud/.github/main/resources/img/organisation-logo-small.png" alt="hauke.cloud logo" width="109" height="123" align="right">

A Kubernetes operator that extracts measurements (moisture, water, temperature, etc.) from MQTT/Zigbee sensors and represents them as Kubernetes Custom Resources. Built with kubebuilder, this operator provides a declarative, cloud-native way to manage IoT sensors and their data.

## Features

- **Declarative Configuration**: Define MQTT bridges and devices using Kubernetes CRDs
- **Multi-Ecosystem Support**: Works with Zigbee2MQTT, Tasmota, and generic MQTT devices
- **Automatic Discovery**: Continuously discovers devices from Zigbee2MQTT bridges
- **Secure Credentials**: Store MQTT credentials in Kubernetes Secrets
- **Device Management**: Enrich discovered devices with custom names, locations, and metadata
- **Real-time Monitoring**: Track device availability, battery levels, and signal quality
- **Measurement Storage**: Store measurements and provide interfaces for database integration
- **Command Support**: Send commands back to devices through MQTT

## Architecture

### Custom Resources

#### MQTTBridge
Represents an MQTT broker connection. The operator connects to configured bridges, subscribes to device topics, and manages device discovery.

**Key Features:**
- Host, port, and TLS configuration
- Credentials stored in Kubernetes Secrets
- Automatic reconnection
- Device discovery enable/disable
- Connection status monitoring

#### Device
Represents a discovered sensor/actuator. The operator creates these resources automatically during discovery, and users can enrich them with metadata.

**Operator-Managed Fields:**
- `spec.bridgeRef`: Reference to parent MQTTBridge
- `spec.ieeeAddr`: Unique IEEE address from Zigbee
- `status.modelId`: Device model
- `status.manufacturer`: Device manufacturer
- `status.capabilities`: List of sensors/actuators
- `status.lastMeasurement`: Latest measurement data
- `status.batteryLevel`: Battery percentage
- `status.linkQuality`: Signal strength

**User-Configurable Fields:**
- `spec.friendlyName`: Human-readable name
- `spec.location`: Physical location
- `spec.room`: Room grouping
- `spec.disabled`: Disable measurement processing
- `spec.metadataLabels`: Custom key-value pairs

#### Database
Represents a TimescaleDB/PostgreSQL connection for persisting sensor measurements. Multiple databases can be configured to handle different sensor types.

**Key Features:**
- Password or client certificate authentication
- SSL/TLS with configurable verification modes
- Connection pooling
- Automatic batching for performance
- Sensor type mapping (moisture, power, solar, etc.)
- GORM integration for automatic table management

**Configuration Fields:**
- `spec.host`, `spec.port`, `spec.database`: Connection details
- `spec.username`: Database username
- `spec.passwordSecretRef` or `spec.clientCertSecretRef`: Authentication
- `spec.sslMode`: SSL verification level
- `spec.sensorType`: Route specific sensor types to this database
- `spec.batchSize`, `spec.batchTimeout`: Performance tuning

## Getting Started

### Prerequisites

- Kubernetes cluster (1.28+)
- kubectl configured
- MQTT broker (e.g., Mosquitto)
- Zigbee2MQTT or Tasmota (for Zigbee devices)
- TimescaleDB/PostgreSQL (optional, for persistent storage)

### Installation

**Prerequisites:**
- MQTTBridge and Device CRDs must be installed by the external device management controller
- This operator only manages the Schedule CRD

The operator automatically installs/upgrades the Schedule CRD at startup by default.

1. **Deploy the operator:**

Development mode (runs locally, installs Schedule CRD automatically):
```bash
make run
```

Production deployment:
```bash
make docker-build docker-push IMG=<your-registry>/mqtt-sensor-exporter:tag
make deploy IMG=<your-registry>/mqtt-sensor-exporter:tag
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

1. **Create MQTT credentials secret:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mqtt-credentials
  namespace: default
type: Opaque
stringData:
  username: "your-mqtt-username"
  password: "your-mqtt-password"
```

```bash
kubectl apply -f mqtt-credentials.yaml
```

2. **Create an MQTTBridge:**

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: MQTTBridge
metadata:
  name: home-zigbee
  namespace: default
spec:
  host: "mqtt.local"
  port: 1883
  credentialsSecretRef:
    name: mqtt-credentials
  topicPrefix: "zigbee2mqtt"
  discoveryEnabled: true
```

```bash
kubectl apply -f mqttbridge.yaml
```

3. **Check connection status:**

```bash
kubectl get mqttbridges
```

4. **View discovered devices:**

```bash
kubectl get devices
```

5. **Enrich a device with metadata:**

```bash
kubectl edit device <device-name>
```

Add your custom fields:
```yaml
spec:
  friendlyName: "Living Room Temperature Sensor"
  location: "Living Room"
  room: "living-room"
  metadataLabels:
    zone: "ground-floor"
    type: "climate"
```

### Configuration Examples

#### TLS-Enabled Bridge

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: MQTTBridge
metadata:
  name: secure-bridge
spec:
  host: "mqtt.example.com"
  port: 8883
  credentialsSecretRef:
    name: mqtt-credentials
  topicPrefix: "zigbee2mqtt"
  tls:
    enabled: true
    insecureSkipVerify: false
    caSecretRef:
      name: mqtt-ca-cert
```

#### Multiple Bridges

You can configure multiple bridges for different MQTT brokers or Zigbee coordinators:

```bash
kubectl apply -f bridge-ground-floor.yaml
kubectl apply -f bridge-first-floor.yaml
kubectl apply -f bridge-garage.yaml
```

#### Database Configuration

Configure TimescaleDB for persistent storage:

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: Database
metadata:
  name: moisture-db
spec:
  host: "timescaledb.default.svc.cluster.local"
  port: 5432
  database: "moisture_sensors"
  username: "sensor_writer"
  passwordSecretRef:
    name: db-credentials
  sensorType: "moisture"
  sslMode: "require"
  maxConnections: 10
  batchSize: 100
```

Multiple databases for different sensor types:

```bash
kubectl apply -f moisture-db.yaml
kubectl apply -f power-db.yaml
kubectl apply -f solar-db.yaml
```

See [DATABASE_QUICKSTART.md](DATABASE_QUICKSTART.md) for detailed setup instructions.

### Monitoring Devices

View all devices with their status:
```bash
kubectl get devices -o wide
```

Get detailed device information:
```bash
kubectl describe device <device-name>
```

Watch for device updates:
```bash
kubectl get devices -w
```

## Integration Points

### Database Storage

The operator provides hooks for storing measurements in databases. Extend the `MessageCallback` in `internal/mqtt/manager.go` to implement your storage logic:

```go
// Example: Store to PostgreSQL, InfluxDB, TimescaleDB, etc.
func (r *DeviceReconciler) handleMeasurement(ctx context.Context, 
    bridgeName types.NamespacedName, 
    ieeeAddr string, 
    payload map[string]interface{}) {
    // Your database logic here
}
```

### Command Interface

Send commands to devices using the MQTT manager:

```go
manager.PublishCommand(namespace, bridgeName, ieeeAddr, map[string]interface{}{
    "state": "ON",
    "brightness": 255,
})
```

## Development

### Project Structure

```
.
├── api/v1alpha1/          # CRD definitions
│   ├── mqttbridge_types.go
│   └── device_types.go
├── internal/
│   ├── controller/        # Reconciliation logic
│   │   ├── mqttbridge_controller.go
│   │   └── device_controller.go
│   └── mqtt/             # MQTT client management
│       └── manager.go
├── config/               # Kubernetes manifests
│   ├── crd/             # Generated CRDs
│   ├── rbac/            # RBAC rules
│   └── samples/         # Example resources
└── cmd/                 # Main entry point
```

### Building

```bash
# Generate code and manifests
make generate
make manifests

# Run tests
make test

# Build binary
make build

# Build and push Docker image
make docker-build docker-push IMG=<registry>/mqtt-sensor-exporter:tag
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

## Roadmap

- [ ] Measurement CRD for Kubernetes-native data storage
- [ ] Prometheus metrics export
- [ ] Grafana dashboard templates
- [ ] Device group management
- [ ] Alerting on device unavailability
- [ ] Historical data queries
- [ ] Device firmware update support
- [ ] Web UI for device management

## Contributing

To become a contributor, please check out the [CONTRIBUTING](CONTRIBUTING.md) file.

## License

This Project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Contact

For any inquiries or support requests, please open an issue in this repository or contact us at [contact@hauke.cloud](mailto:contact@hauke.cloud).

