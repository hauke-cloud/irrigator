# MQTT Sensor Exporter Helm Chart

This Helm chart deploys the MQTT Sensor Exporter Kubernetes operator.

## Overview

The MQTT Sensor Exporter is a Kubernetes operator that manages IoT sensor data from MQTT brokers (Tasmota, Zigbee2MQTT) and stores measurements in TimescaleDB.

## Prerequisites

- Kubernetes 1.28+
- Helm 3.0+

## Installation

### Install from OCI registry

```bash
helm install irrigator \
  oci://ghcr.io/hauke-cloud/charts/irrigator \
  --version 0.1.0
```

### Install from local chart

```bash
helm install irrigator ./deployments/helm/irrigator
```

### Install with custom values

```bash
helm install irrigator ./deployments/helm/irrigator \
  --values my-values.yaml
```

## Configuration

### Basic Configuration

```yaml
# values.yaml
image:
  repository: ghcr.io/hauke-cloud/irrigator
  tag: "1.0.0"

resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi

operator:
  installCRDs: true
  leaderElection: true
```

### Disable Automatic CRD Installation

If you want to manage CRDs separately:

```yaml
operator:
  installCRDs: false

crds:
  install: false
```

### Enable Monitoring

```yaml
monitoring:
  serviceMonitor:
    enabled: true
    interval: 30s
```

## Parameters

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Image repository | `ghcr.io/hauke-cloud/irrigator` |
| `image.tag` | Image tag | Chart appVersion |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

### Operator Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.installCRDs` | Auto-install CRDs at startup | `true` |
| `operator.leaderElection` | Enable leader election | `true` |
| `operator.metrics.enabled` | Enable metrics endpoint | `true` |
| `operator.metrics.port` | Metrics port | `8443` |
| `operator.health.port` | Health probe port | `8081` |

### Resource Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `64Mi` |

### Logging Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `logging.level` | Log level (debug, info, warn, error) | `info` |
| `logging.format` | Log format (json, console) | `json` |

## Usage

After installation, create MQTTBridge and Database resources:

### Create MQTT Credentials

```bash
kubectl create secret generic mqtt-credentials \
  --from-literal=username='mqtt-user' \
  --from-literal=password='mqtt-password'
```

### Create Database Credentials

```bash
kubectl create secret generic db-credentials \
  --from-literal=password='db-password'
```

### Deploy MQTTBridge

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: MQTTBridge
metadata:
  name: tasmota-bridge
spec:
  host: "mqtt.local"
  port: 1883
  credentialsSecretRef:
    name: mqtt-credentials
  deviceType: "tasmota"
  topics:
    - topic: "tele/+/SENSOR"
      type: "telemetry"
```

### Deploy Database

```yaml
apiVersion: mqtt.hauke.cloud/v1alpha1
kind: Database
metadata:
  name: irrigation-db
spec:
  host: "timescaledb.default.svc.cluster.local"
  port: 5432
  database: "irrigation_sensors"
  username: "irrigation_writer"
  passwordSecretRef:
    name: db-credentials
  supportedSensorTypes:
    - "irrigation"
```

## Upgrading

```bash
helm upgrade irrigator \
  oci://ghcr.io/hauke-cloud/charts/irrigator \
  --version 0.2.0
```

## Uninstalling

```bash
helm uninstall irrigator
```

**Note:** CRDs are not automatically removed. To remove them:

```bash
kubectl delete crd mqttbridges.mqtt.hauke.cloud
kubectl delete crd devices.mqtt.hauke.cloud
kubectl delete crd databases.mqtt.hauke.cloud
```

## Documentation

- [Project README](https://github.com/hauke-cloud/irrigator)
- [Database CRD](../../../DATABASE_CRD.md)
- [Device Sensor Types](../../../DEVICE_SENSOR_TYPE.md)
- [Sensor Type Mapping](../../../SENSOR_TYPE_MAPPING.md)

## Support

For issues and questions, please open an issue at:
https://github.com/hauke-cloud/irrigator/issues
