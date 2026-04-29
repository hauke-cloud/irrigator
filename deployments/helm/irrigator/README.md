# Irrigator Helm Chart

Kubernetes operator for automated irrigation scheduling with Tasmota valve devices.

## Prerequisites

- Kubernetes 1.29+
- Helm 3.0+
- mqtt-sensor-exporter deployed (for alerts API)
- Client certificates for sensor exporter API access

## Installation

### 1. Create Client Certificate Secret

First, create a secret containing the client certificates for accessing the sensor exporter API:

```bash
kubectl create secret generic irrigator-sensor-exporter-client-cert \
  --from-file=tls.crt=path/to/client.crt \
  --from-file=tls.key=path/to/client.key \
  --from-file=ca.crt=path/to/ca.crt \
  -n your-namespace
```

### 2. Install the Chart

```bash
helm install irrigator ./deployments/helm/irrigator \
  --namespace your-namespace \
  --create-namespace
```

Or with custom values:

```bash
helm install irrigator ./deployments/helm/irrigator \
  --namespace your-namespace \
  --values your-values.yaml
```

## Configuration

### Essential Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `sensorExporterAPI.url` | URL of sensor exporter API | `https://mqtt-sensor-exporter.mqtt.svc.cluster.local:8443` |
| `sensorExporterAPI.secretName` | Secret containing client certificates | `irrigator-sensor-exporter-client-cert` |

### Advanced Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.installCRDs` | Auto-install CRDs at startup | `true` |
| `operator.leaderElection` | Enable leader election | `true` |
| `logging.level` | Log level (debug/info/warn/error) | `info` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |

See [values.yaml](values.yaml) for all available options.

## Upgrading

### From v0.1.x to v0.2.x

**⚠️ Breaking Change**: API group changed from `mqtt.hauke.cloud` to `iot.hauke.cloud`

See [MIGRATION.md](../MIGRATION.md) for detailed upgrade instructions.

## Usage

### Create a Schedule

```yaml
apiVersion: iot.hauke.cloud/v1alpha1
kind: Schedule
metadata:
  name: morning-watering
  namespace: default
spec:
  deviceName: garden-valve-1
  cronExpression: "0 6 * * *"  # Every day at 6:00 AM
  durationSeconds: 1800         # 30 minutes
  enabled: true
  timeZone: "Europe/Berlin"
  executionConditions:
    - sensorType: water_level
      alert: false
      message: "Water tank level is low"
```

### Check Schedule Status

```bash
# List all schedules
kubectl get schedules -A

# Get detailed status
kubectl describe schedule morning-watering

# Watch schedule execution
kubectl get schedule morning-watering -w
```

## Architecture

The irrigator operator:
- **Watches**: Device and MQTTBridge CRs (read-only, managed by mqtt-device-manager)
- **Manages**: Schedule CRs (full CRUD)
- **Controls**: Valve devices via MQTT commands
- **Checks**: Execution conditions via sensor exporter alerts API

```
┌─────────────────┐
│   Irrigator     │
│   Operator      │
└────────┬────────┘
         │
    ┌────┴────────────────────────────┐
    │                                 │
    ▼                                 ▼
┌─────────────┐            ┌──────────────────┐
│  MQTT       │            │  Sensor Exporter │
│  Bridge     │            │  API (mTLS)      │
└─────────────┘            └──────────────────┘
    │                                 │
    ▼                                 ▼
┌─────────────┐            ┌──────────────────┐
│  Tasmota    │            │  Alerts          │
│  Valves     │            │  Endpoint        │
└─────────────┘            └──────────────────┘
```

## Troubleshooting

### Check Operator Logs

```bash
kubectl logs -n your-namespace deployment/irrigator -f
```

### Verify API Connectivity

```bash
kubectl logs -n your-namespace deployment/irrigator | grep -i "alerts\|sensor.*exporter"
```

### Check RBAC Permissions

```bash
kubectl auth can-i list schedules.iot.hauke.cloud --as=system:serviceaccount:your-namespace:irrigator
```

### Certificate Issues

```bash
# Verify secret exists
kubectl get secret irrigator-sensor-exporter-client-cert -n your-namespace

# Check certificate mount
kubectl describe pod -n your-namespace -l control-plane=controller-manager | grep -A 5 "Mounts:"
```

## Uninstallation

```bash
helm uninstall irrigator -n your-namespace
```

**Note**: By default, CRDs are kept after uninstallation. To remove them:

```bash
kubectl delete crd schedules.iot.hauke.cloud
```

## Support

- GitHub Issues: https://github.com/hauke-cloud/irrigator/issues
- Email: contact@hauke.cloud

## License

Apache License 2.0
