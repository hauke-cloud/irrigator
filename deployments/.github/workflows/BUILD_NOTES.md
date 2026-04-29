# Build Process Notes

## CRD Embedding

The project embeds CRD YAML files into the binary for automatic CRD installation at runtime. This requires special handling during the build process.

### Directory Structure

```
config/crd/bases/                    # Source CRD files (version controlled)
├── mqtt.hauke.cloud_mqttbridges.yaml
├── mqtt.hauke.cloud_devices.yaml
└── mqtt.hauke.cloud_databases.yaml

cmd/crd/                             # Embedded CRD files (generated, .gitignored)
├── mqtt.hauke.cloud_mqttbridges.yaml
├── mqtt.hauke.cloud_devices.yaml
└── mqtt.hauke.cloud_databases.yaml
```

### Why cmd/crd/ is .gitignored

The `cmd/crd/` directory contains **copies** of the CRD files from `config/crd/bases/`. These files are:
- Generated during the build process
- Not committed to version control
- Created by running `make copy-crds`

### Build Process

#### Local Development

```bash
# Generate and copy CRDs
make copy-crds

# Build the binary
make build

# Or use the combined command
go run ./cmd/...
```

The `make copy-crds` target:
1. Runs `controller-gen` to generate CRDs in `config/crd/bases/`
2. Copies CRD files to `cmd/crd/` for Go embedding

#### Docker Build

The Dockerfile handles this automatically:

```dockerfile
# Copy CRD base files to cmd/crd for embedding
RUN mkdir -p cmd/crd && \
    cp config/crd/bases/*.yaml cmd/crd/ 2>/dev/null || true
```

The `.dockerignore` file allows `config/crd/bases/*.yaml` to be included in the build context.

#### GitHub Actions Workflow

The workflow runs `make copy-crds` before building or running tests:

```yaml
- name: Generate manifests and copy CRDs
  run: make manifests generate copy-crds

- name: Run go vet
  run: go vet ./...
```

### Troubleshooting

#### Error: pattern crd/mqtt.hauke.cloud_*.yaml: no matching files found

**Cause:** The `cmd/crd/` directory is empty or missing.

**Solution:**
```bash
make copy-crds
```

#### CRD files are out of sync

**Cause:** Changes to API types haven't been regenerated.

**Solution:**
```bash
make manifests generate copy-crds
```

#### Docker build fails with "no such file or directory"

**Cause:** `.dockerignore` is preventing CRD files from being copied.

**Solution:** Ensure `.dockerignore` includes:
```
!config/crd/bases/*.yaml
```

### CI/CD Workflow

The workflow ensures CRDs are always generated before:
1. Running `go vet`
2. Running tests
3. Building the binary
4. Building the Docker image

### Best Practices

1. **Always run `make copy-crds`** before local development
2. **Never commit `cmd/crd/`** - it's in `.gitignore` for a reason
3. **Regenerate CRDs** after modifying API types:
   ```bash
   make manifests generate copy-crds
   ```
4. **The Makefile handles this** - use `make build` or `make run`

### Verification

To verify your build setup:

```bash
# Clean and rebuild
rm -rf cmd/crd
make copy-crds
go vet ./...
go build ./cmd/...
```

All steps should succeed without errors.
