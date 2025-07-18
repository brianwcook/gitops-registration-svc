# GitHub Actions CI/CD Pipeline

This repository includes a comprehensive CI/CD pipeline using GitHub Actions to ensure code quality, security, and reliable deployments.

## 🚀 Workflows Overview

### 1. **Main CI/CD Pipeline** (`.github/workflows/ci.yml`)

**Triggers:** Push to `main`/`develop`, Pull Requests  
**Purpose:** Complete validation pipeline

**Jobs:**
- **Unit Tests** - Fast Go unit tests with coverage reporting
- **Build Image** - Multi-platform container builds with caching
- **Integration Tests** - Full K8s testing across multiple versions
- **Security Scan** - Trivy vulnerability scanning
- **Lint & Format** - Code quality checks with golangci-lint
- **Release** - Automated tagging and release notes (main branch only)

### 2. **PR Validation** (`.github/workflows/pr-checks.yml`)

**Triggers:** Pull Request events  
**Purpose:** Fast feedback for PR validation

**Jobs:**
- **Quick Validation** - Formatting, build, unit tests, coverage
- **Dockerfile Lint** - Hadolint security and best practices
- **YAML Lint** - Configuration file validation
- **Security Check** - Gosec static security analysis
- **PR Summary** - Consolidated status report

### 3. **Manual Testing** (`.github/workflows/manual-test.yml`)

**Triggers:** Manual dispatch only  
**Purpose:** On-demand testing and debugging

**Features:**
- Configurable test types (unit, integration, debug)
- Custom container image override
- Multiple Kubernetes versions
- Debug mode with extra diagnostics
- Optional cluster preservation for inspection

## 📊 Test Matrix

| Workflow | Go Version | K8s Versions | Platforms | Coverage |
|----------|------------|--------------|-----------|----------|
| Main CI | 1.21 | v1.29.2, v1.30.0 | linux/amd64, linux/arm64 | ✅ |
| PR Checks | 1.21 | N/A | linux/amd64 | ✅ |
| Manual | 1.21 | v1.29.2, v1.30.0, v1.31.0 | linux/amd64 | ✅ |

## 🔧 Configuration

### Environment Variables

- `REGISTRY`: Container registry (default: `ghcr.io`)
- `IMAGE_NAME`: Image name based on repository

### Required Secrets

- `GITHUB_TOKEN`: Automatically provided by GitHub
- `CODECOV_TOKEN`: Optional, for coverage reporting

### Repository Settings

1. **Branch Protection** (recommended):
   ```yaml
   Required status checks:
   - Unit Tests
   - Quick Validation
   - Integration Tests (v1.29.2)
   - Integration Tests (v1.30.0)
   ```

2. **Security Settings**:
   - Enable Dependabot alerts
   - Enable code scanning (Trivy & Gosec)
   - Require signed commits (optional)

## 🏃‍♂️ Usage

### For Pull Requests

1. **Create PR** → Triggers PR validation workflow
2. **Wait for checks** → ✅ All checks must pass
3. **Merge** → Triggers full CI pipeline on target branch

### For Emergency Testing

1. Go to **Actions** tab in GitHub
2. Select **Manual Testing & Debugging**
3. Click **Run workflow**
4. Configure options:
   - **Test Type**: `unit` | `integration` | `both` | `debug`
   - **Image Override**: Custom container image (optional)
   - **K8s Version**: Target Kubernetes version
   - **Debug Mode**: Enable extra logging
   - **Keep Cluster**: Preserve for inspection

### For Local Development

```bash
# Run the same checks locally
make test-unit                    # Unit tests
make lint                        # Linting (requires golangci-lint)
make test-integration-full       # Full integration tests
```

## 📈 Monitoring & Observability

### Test Coverage

- **Minimum**: 60% coverage required
- **Reporting**: Codecov integration
- **Trend**: Coverage reports on all PRs

### Build Artifacts

- **Container Images**: Tagged and pushed to GHCR
- **Test Reports**: Available in Actions artifacts
- **Security Scans**: Uploaded to GitHub Security tab

### Notification Channels

- **PR Comments**: Automated status updates
- **Email**: GitHub notifications for failed builds
- **Slack**: Configure webhooks for team notifications

## 🔍 Debugging Failed Builds

### Unit Test Failures

1. Check test output in Actions logs
2. Run locally: `make test-unit`
3. Check coverage reports in artifacts

### Integration Test Failures

1. Download diagnostic artifacts from failed runs
2. Check cluster logs in `artifacts/service-logs.txt`
3. Use manual workflow with debug mode enabled
4. Review Kubernetes events in artifacts

### Build Failures

1. Check Docker build logs
2. Verify Dockerfile syntax with `hadolint`
3. Test multi-platform builds locally

### Security Scan Failures

1. Review Trivy/Gosec reports in Security tab
2. Update dependencies with Dependabot
3. Fix code issues identified by scanners

## 🛠 Customization

### Adding New Checks

Edit `.github/workflows/ci.yml`:

```yaml
new-check:
  name: Custom Check
  runs-on: ubuntu-latest
  steps:
  - uses: actions/checkout@v4
  - name: Run custom check
    run: ./scripts/custom-check.sh
```

### Modifying Test Matrix

Update the `strategy.matrix` section:

```yaml
strategy:
  matrix:
    k8s-version: ['v1.29.2', 'v1.30.0', 'v1.31.0']
    go-version: ['1.21', '1.22']
```

### Custom Linting Rules

Edit `.golangci.yml` to modify linting configuration.

## 📚 Best Practices

1. **Keep workflows fast** - Use caching and parallel jobs
2. **Fail fast** - Run quick checks before expensive operations
3. **Clear feedback** - Provide actionable error messages
4. **Security first** - Scan dependencies and container images
5. **Documentation** - Keep this README updated with changes

## 🔄 Maintenance

### Regular Tasks

- **Weekly**: Review Dependabot PRs
- **Monthly**: Update action versions
- **Quarterly**: Review and update K8s test matrix

### Workflow Updates

When updating workflows:
1. Test in a feature branch first
2. Use manual workflow to validate changes
3. Update documentation as needed
4. Monitor first few runs after merge

## 📞 Support

- **Issues**: Create GitHub issues for workflow problems
- **Discussions**: Use GitHub Discussions for questions
- **Emergency**: Use manual workflow for urgent debugging 