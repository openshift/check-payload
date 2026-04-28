# Contributor Guide

## Development Workflow

Before submitting changes, run:

```bash
make verify  # Runs all verification checks
make test    # Runs test suite
```

The `verify` target includes:

- `verify-space`: Ensures no trailing whitespace
- `verify-generate`: Verifies `go generate ./internal/types` is up to date
- `verify-golangci`: Runs golangci-lint with configured linters (gofumpt, errorlint, unconvert, unparam, revive)

All checks must pass before submitting PRs.

## Config.toml Exceptions

**IMPORTANT: Before adding any exception, verify that the exception is truly necessary.**

Steps:

1. Ask user to verify: Confirm exception is necessary and the non-compliant item cannot be fixed/removed
2. Add justification comment: Explain why the exception is needed
3. Link to issues: Reference bug numbers (e.g., `# See OCPBUGS-36541.`) when available
4. Use appropriate scope:
   - `[[rpm.PACKAGE.ignore]]` for RPM-level
   - `[[payload.COMPONENT.ignore]]` for payload components
   - `[[tag.TAG.ignore]]` for tags
5. Specify error type: Use exact error name:
   - Binary: `ErrNotDynLinked`, `ErrGoMissingTag`, `ErrGoNotCgoEnabled`, `ErrLibcryptoMissing`
   - OS: `ErrOSNotCertified` (images not using certified distributions like UBI); `ErrFipsArtifactMissing` (certified artifact not found); `ErrFipsArtifactVersionLow` (artifact version below minimum)
   - Library: `ErrLibcryptoSoMissing` (missing OpenSSL in container images)
6. Use files or dirs: Specify `files = [...]` or `dirs = [...]` with absolute paths

Examples:

Binary exception:

```toml
[[rpm.runc.ignore]]
# See OCPBUGS-36541.
error = "ErrGoMissingSymbols"
files = ["/usr/bin/runc"]
```

OS certification exception:

```toml
[[tag.rhel-coreos.ignore]]
# RHCOS transport image - ignore OS certification check
# The rhel-coreos tag is used to transport the base OS image that OpenShift nodes run on.
error = "ErrOSNotCertified"
tags = ["rhel-coreos"]
```

Note: Java validation uses `java_fips_disabled_algorithms` configuration instead of exception rules.

## Releases

Releases are managed via git tags using semantic versioning (e.g., `0.3.11`):

1. Update `CHANGELOG.md` with changes
2. Create and push a git tag: `git tag -s v0.x.x && git push origin v0.x.x`
3. Version information is embedded at build time via `git describe --tags`

Version-specific configurations are embedded from `dist/releases/4.x/config.toml` during build. `fips_validation_mode` and `fips_certified_modules` live in root `config.toml`; version configs handle exceptions only.

## FIPS Module Validation

`fips_validation_mode`: `allowlist` = OS check only (`certified_distributions`). `module` = also enforce `fips_certified_modules`.

Each entry maps a crypto stack (`module`) to its `certified_artifact`, with optional `certified_artifact_min_version` and `certified_artifact_paths` for file-based fallback. Artifact presence verified via RPM query or file path.

Flow (module mode): OS allowlist check runs early. After binary scanning, detected crypto modules are matched against config entries and each artifact is validated.

## General Contribution Guidelines

- Follow Go best practices and project code patterns
- Ensure code is testable with clear separation of concerns
- Keep code self-documenting (avoid comments unless complexity warrants them)
- Run `make verify` and `make test` before submitting
- Use active voice in documentation
