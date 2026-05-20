# Test fixtures

ELF binaries used by `TestReadTable` and `TestRunLocalScan`. Each fixture
covers a specific combination of architecture, Go version, link mode, and
FIPS configuration.

All fixtures are built from `main.go` using `build_fixture.sh`, which runs
Red Hat's `go-toolset` container images with zig for cross-compilation.

## When to rebuild

Only when you change `main.go`, `Containerfile.builder`, `build_fixture.sh`,
or need a different Go version. Normal development does not require rebuilding -
just run `make test`.

## How to rebuild

```bash
# Remove cached builder images (one per Go version)
podman images --filter reference=check-payload-fixture-builder -q \
  | xargs -r podman rmi -f

# Rebuild all fixtures
./build_fixture.sh --goarch=s390x --cgo --go-image=1.24 --output=go124_s390x_app
./build_fixture.sh --buildmode=pie --go-image=1.24 --output=go124_internal_pie_amd64_app
./build_fixture.sh --goarch=s390x --cgo --buildmode=pie --fips --output=pie_go126_s390x_app
./build_fixture.sh --cgo --fips --go-image=1.20 --output=fips_compliant_app
./build_fixture.sh --fips --output=go-native-fips-app
./build_fixture.sh --goarch=ppc64le --cgo --go-image=1.24 --output=go124_ppc64le_app
./build_fixture.sh --cgo --buildmode=pie --go-image=1.24 --output=go124_external_pie_amd64_app

# Update mock directory copies (symlinked dirs stay in sync automatically)
cp pie_go126_s390x_app mock_unpacked_dir_pie_s390x/usr/pie_go126_s390x_app
cp fips_compliant_app mock_unpacked_dir-1/usr/fips_compliant_app
cp go-native-fips-app mock_native_fips/usr/bin/go-native-fips-app

# Commit rebuilt fixtures, then verify
git add -u test/resources/
make test verify
```

Each build produces unique BuildIDs, so rebuilt fixtures must be committed
before `make verify` passes.

## Fixture matrix

| Fixture | Arch | Go | FIPS | ELF section | Tests |
|---|---|---|---|---|---|
| `fips_compliant_app` | amd64 | 1.20 | boringcrypto | `.gopclntab` | symbol detection |
| `go124_s390x_app` | s390x | 1.24 | - | `.gopclntab` | false LE magic match |
| `go124_internal_pie_amd64_app` | amd64 | 1.24 | - | `.data.rel.ro.gopclntab` | internal PIE layout |
| `go124_external_pie_amd64_app` | amd64 | 1.24 | - | `.data.rel.ro` | magic scanning fallback |
| `go124_ppc64le_app` | ppc64le | 1.24 | - | `.gopclntab` | quantum=4 LE |
| `pie_go126_s390x_app` | s390x | 1.26 | native | `.gopclntab` | Go 1.26 PIE layout |
| `go-native-fips-app` | amd64 | 1.26 | native | `.gopclntab` | native FIPS validation |

## Mock directories

Integration tests (`TestRunLocalScan`) use mock unpacked directories that
simulate container image layout. Directories with symlinked binaries
(`mock_unpacked_dir_go124_s390x/`, etc.) stay in sync automatically.
Directories with copied binaries need manual updates after rebuilds.
