# GitHub Actions Workflows

## CI/CD Pipeline (`ci.yml`)

This workflow provides comprehensive continuous integration and deployment for the MQTT Sensor Exporter.

### Triggers

- **Push to branches**: `main`, `develop`
- **Pull requests**: targeting `main` or `develop`
- **Tags**: Any tag starting with `v*` (e.g., `v1.0.0`, `v0.2.1-alpha`)

### Jobs

#### 1. Test Job

Runs unit tests and generates coverage reports.

**Steps:**
- Checkout code
- Set up Go 1.25
- Download dependencies
- Run `go fmt` check
- Run `go vet`
- Generate manifests and code
- Execute tests with coverage
- Upload coverage to Codecov (optional)

#### 2. Lint Job

Runs static code analysis.

**Steps:**
- Checkout code
- Set up Go 1.25
- Run golangci-lint v2.8.0 with 5-minute timeout

#### 3. Build Image Job

Builds multi-architecture OCI container images.

**Steps:**
- Set up QEMU for multi-arch builds
- Set up Docker Buildx
- Log in to GitHub Container Registry (ghcr.io)
- Extract metadata and generate tags
- Build and push images for `linux/amd64` and `linux/arm64`
- Use GitHub Actions cache for faster builds

**Image Tags Generated:**
- Branch name (e.g., `main`, `develop`)
- PR reference (e.g., `pr-123`)
- Semver tags (e.g., `1.0.0`, `1.0`, `1`)
- SHA-based tags (e.g., `main-abc1234`)
- `latest` tag for default branch

**Outputs:**
- `image-digest`: SHA256 digest of the built image
- `image-tags`: List of all tags applied

#### 4. Build Helm Job

Packages the Helm chart.

**Steps:**
- Checkout code
- Set up Helm v3.14.0
- Lint Helm chart
- Package chart to `dist/` directory
- Upload chart as artifact (30-day retention)

#### 5. Release Job

Creates GitHub releases for tagged builds.

**Triggers:** Only runs when a tag starting with `v` is pushed

**Steps:**
- Download Helm chart artifact
- Extract version from tag
- Generate release notes from `CHANGES.md` or git log
- Create GitHub release with:
  - Release notes
  - Helm chart package attached
  - Draft: false
  - Prerelease: true if tag contains `alpha`, `beta`, or `rc`
- Update Helm repository index on `gh-pages` branch

**Release Notes:**
- Extracted from `CHANGES.md` if available
- Falls back to git log between tags

#### 6. Security Scan Job

Scans container images for vulnerabilities.

**Triggers:** Only runs for push events (not PRs)

**Steps:**
- Run Trivy vulnerability scanner
- Generate SARIF report
- Upload results to GitHub Security tab

**Permissions:**
- `security-events: write`
- `contents: read`

### Environment Variables

- `REGISTRY`: `ghcr.io` (GitHub Container Registry)
- `IMAGE_NAME`: Full repository name (e.g., `hauke-cloud/irrigator`)
- `GO_VERSION`: `1.25`

### Permissions

The workflow uses fine-grained permissions:

- **Test/Lint**: Read-only access to code
- **Build**: Write access to packages, read content
- **Release**: Write access to contents and packages
- **Security Scan**: Write to security events

### Usage Examples

#### Trigger CI on Pull Request

```bash
git checkout -b feature/my-feature
git push origin feature/my-feature
# Open PR on GitHub
```

#### Create a Release

```bash
# Update version in Chart.yaml and other files
git tag v1.2.3
git push origin v1.2.3
# Workflow will create release automatically
```

#### Manual Testing

```bash
# Run tests locally (same as CI)
make test

# Run linting locally
make lint

# Build image locally
make docker-build IMG=test:local
```

### Artifacts

#### Helm Chart
- **Location**: Downloadable from workflow run or GitHub release
- **Retention**: 30 days for workflow artifacts, permanent for releases
- **Format**: `.tgz` package

#### Container Images
- **Registry**: ghcr.io/hauke-cloud/irrigator
- **Platforms**: linux/amd64, linux/arm64
- **Retention**: Per GitHub Container Registry retention policy

### Helm Repository

Released charts are indexed on the `gh-pages` branch, creating a Helm repository:

```bash
# Add the Helm repository
helm repo add irrigator https://hauke-cloud.github.io/irrigator/

# Install the chart
helm install my-release irrigator/irrigator
```

### Secrets Required

#### Optional Secrets

- `CODECOV_TOKEN`: For uploading coverage reports to Codecov

#### Automatic Secrets (provided by GitHub)

- `GITHUB_TOKEN`: Automatically provided for authentication

### Customization

#### Modify Target Branches

```yaml
on:
  push:
    branches:
      - main
      - staging  # Add staging
```

#### Change Supported Platforms

```yaml
platforms: linux/amd64,linux/arm64,linux/arm/v7
```

#### Adjust Go Version

```yaml
env:
  GO_VERSION: '1.26'
```

### Troubleshooting

#### Failed Tests
Check the test job logs for specific failures. Run `make test` locally to reproduce.

#### Image Build Fails
- Verify Dockerfile syntax
- Check multi-arch support
- Review Docker Buildx logs

#### Release Not Created
- Ensure tag follows `v*` pattern
- Check if tag was pushed: `git push origin v1.0.0`
- Verify workflow permissions

#### Helm Chart Issues
- Run `helm lint deployments/helm/irrigator` locally
- Check Chart.yaml and values.yaml syntax

### Best Practices

1. **Always create tags from main/master**: Ensure the release is based on stable code
2. **Use semantic versioning**: Follow `vMAJOR.MINOR.PATCH` pattern
3. **Update CHANGES.md**: Document changes before tagging
4. **Test locally first**: Run `make test` and `make lint` before pushing
5. **Review PR checks**: Ensure all checks pass before merging

### Monitoring

- **Workflow runs**: View in Actions tab on GitHub
- **Container images**: Check ghcr.io packages
- **Security alerts**: Review Security tab for vulnerabilities
- **Coverage**: View on Codecov (if configured)
