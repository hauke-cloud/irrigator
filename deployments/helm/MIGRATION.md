# Helm Chart Migration Guide

## API Group Migration: mqtt.hauke.cloud → iot.hauke.cloud

The irrigator Helm chart has been updated to use the new `iot.hauke.cloud` API group for all CRDs (Schedule, Device, MQTTBridge).

### Prerequisites

1. **kubernetes-iot-api CRDs installed**
   - The new CRDs from kubernetes-iot-api must be installed
   - Can be installed by mqtt-sensor-exporter or manually

2. **Client certificates for sensor exporter API**
   - Create a secret with mTLS client certificates
   - Required for alerts API access

### Migration Steps

#### 1. Backup Existing Schedules

```bash
# Backup all existing schedules
kubectl get schedules.mqtt.hauke.cloud -A -o yaml > schedules-backup.yaml
```

#### 2. Create Client Certificate Secret

```bash
# Create secret with client certificates for sensor exporter API
kubectl create secret generic irrigator-sensor-exporter-client-cert \
  --from-file=tls.crt=path/to/client.crt \
  --from-file=tls.key=path/to/client.key \
  --from-file=ca.crt=path/to/ca.crt \
  -n <namespace>
```

#### 3. Update Helm Values

Update your `values.yaml` or override values:

```yaml
sensorExporterAPI:
  # URL of the mqtt-sensor-exporter API
  url: "https://mqtt-sensor-exporter.mqtt.svc.cluster.local:8443"
  # Secret containing client certificates
  secretName: "irrigator-sensor-exporter-client-cert"
  # Certificate file names (if different from defaults)
  certFile: "tls.crt"
  keyFile: "tls.key"
  caFile: "ca.crt"
```

#### 4. Upgrade Helm Release

```bash
# Upgrade the irrigator release
helm upgrade irrigator ./deployments/helm/irrigator \
  -n <namespace> \
  --values your-values.yaml
```

The operator will automatically:
- Install the new `iot.hauke.cloud` Schedule CRD
- Start managing schedules under the new API group

#### 5. Migrate Schedule Resources

Option A: **Automatic migration via script**

```bash
# Convert schedules to new API group
kubectl get schedules.mqtt.hauke.cloud -A -o yaml | \
  sed 's/apiVersion: mqtt.hauke.cloud/apiVersion: iot.hauke.cloud/g' | \
  kubectl apply -f -
```

Option B: **Manual migration**

For each schedule:
1. Get the schedule definition
2. Change `apiVersion: mqtt.hauke.cloud/v1alpha1` to `apiVersion: iot.hauke.cloud/v1alpha1`
3. Apply the updated schedule
4. Delete the old schedule

```bash
# Example for a single schedule
kubectl get schedule my-schedule -n default -o yaml | \
  sed 's/mqtt.hauke.cloud/iot.hauke.cloud/g' | \
  kubectl apply -f -

# Delete old schedule
kubectl delete schedule.mqtt.hauke.cloud my-schedule -n default
```

#### 6. Verify Migration

```bash
# Check new schedules are running
kubectl get schedules.iot.hauke.cloud -A

# Check operator logs
kubectl logs -n <namespace> deployment/irrigator -f

# Verify alerts API connectivity
kubectl logs -n <namespace> deployment/irrigator | grep -i "alerts"
```

#### 7. Clean Up Old CRDs (Optional)

Once all schedules are migrated and working:

```bash
# Remove old Schedule CRD
kubectl delete crd schedules.mqtt.hauke.cloud

# Note: Device and MQTTBridge CRDs should be kept if used by other controllers
```

### Configuration Reference

#### Required Configuration

- `sensorExporterAPI.url`: URL of mqtt-sensor-exporter API
- `sensorExporterAPI.secretName`: Name of secret with client certificates

#### Optional Configuration

- `sensorExporterAPI.certPath`: Mount path for certificates (default: `/etc/sensor-exporter-certs`)
- `sensorExporterAPI.certFile`: Client certificate filename (default: `tls.crt`)
- `sensorExporterAPI.keyFile`: Client key filename (default: `tls.key`)
- `sensorExporterAPI.caFile`: CA certificate filename (default: `ca.crt`)
- `operator.installCRDs`: Auto-install CRDs at startup (default: `true`)

### Troubleshooting

#### Schedules Not Found

```bash
# Check if schedules exist under old API group
kubectl get schedules.mqtt.hauke.cloud -A

# Check if schedules exist under new API group
kubectl get schedules.iot.hauke.cloud -A
```

#### RBAC Errors

Ensure the ClusterRole has permissions for `iot.hauke.cloud` API group:

```bash
kubectl get clusterrole irrigator-manager-role -o yaml | grep -A 5 "iot.hauke.cloud"
```

#### Certificate Issues

```bash
# Verify secret exists and contains required keys
kubectl get secret irrigator-sensor-exporter-client-cert -n <namespace> -o yaml

# Check certificate mount in pod
kubectl describe pod -n <namespace> -l control-plane=controller-manager | grep -A 10 "Mounts:"
```

#### Alerts API Connectivity

```bash
# Check operator logs for alerts API errors
kubectl logs -n <namespace> deployment/irrigator | grep -i "alerts\|sensor.*exporter"

# Test connectivity from pod (if debugging tools available)
kubectl exec -n <namespace> deployment/irrigator -- curl -k --cert /etc/sensor-exporter-certs/tls.crt \
  --key /etc/sensor-exporter-certs/tls.key https://mqtt-sensor-exporter.mqtt.svc.cluster.local:8443/alerts
```

### Rollback

If issues occur, you can rollback:

```bash
# Rollback Helm release
helm rollback irrigator -n <namespace>

# Restore old schedules from backup
kubectl apply -f schedules-backup.yaml
```

### Breaking Changes

1. **API Group Changed**: `mqtt.hauke.cloud/v1alpha1` → `iot.hauke.cloud/v1alpha1`
2. **Sensor Exporter API Required**: Must configure client certificates
3. **ValvePowerState Removed**: Field removed from Schedule status
4. **Measurement Conditions**: Temporarily disabled (only alert-based conditions work)

### New Features

1. **Alerts API Integration**: Execution conditions now use centralized alerts API
2. **Shared CRD Management**: CRDs managed via kubernetes-iot-api
3. **mTLS Authentication**: Secure communication with sensor exporter API
