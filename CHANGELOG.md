# Changelog

## [0.2.19] - 2023-07-11

### Bug Fixes

- Fix remove container create/rm step
- Remove obsoleted requirements
- Use RPM name in node scan
- perf: validaetGoSymbols and skip early 
- perf: compile regexes only once
- perf: isGoExecutable do not use regexp

### Features

- Add node ignores
- Add 4.9, 4.10, 4.11, 4.12, 4.13 config files
- Add warning support and ---fail-on-warnings

## [0.2.18] - 2023-06-30

### Features

* Embed per-version config files, allow to choose one using -V,
  --config-for-version option (for example: `scan -V 4.12 payload ...`)

## [0.2.17] - 2023-06-30

### Bug fixes

- Fixes to -p output

### Features

- Add support for per-tag ignores
- Add config file for 4.12

## [0.2.16] - 2023-06-30

### Bug fixes

- Cleanup Go symbols error message
- Fix PIE executables detection
- GHA-related fixes to CI
- Add LICENSE

### Features

- Add support for per-payload image ignores
- Add exception printer (-p) option
- Configuration: add more exceptions

### Documentation

- CHANGELOG: cleanup
- README: add prereqiusites

## [0.2.15] - 2023-06-26

### Bug Fixes

- Add rhel9 fips symbol

### Features

- Add `sysroot` to filtered directories list

## [0.2.14] - 2023-06-26

### Bug Fixes

- Use file open instead of strings

### Features

- Add `/usr/src/multus-cni/rhel7/bin/multus` to filter list

## [0.2.13] - 2023-06-26

### Bug Fixes

- Check for `fips_mode`

### Features

- Add more binaries to filter list: `/usr/local/bin/catatonit`
- Use rootfs

## [0.2.12] - 2023-06-23

### Bug Fixes

- Fix openssl detection

## [0.2.11] - 2023-06-23

### Features

- Support specifying pull secret for oc adm release info

## [0.2.10] - 2023-06-22

### Features

- Add more binaries to filter list: `grpc_health_probe`

## [0.2.9] - 2023-06-22

### Bug Fixes

- Podman: cleanup container
- Improve memory usage for `node_scan`

### Features

- Use backup entrypoint /bin/sh
- Allow for alternate entrypoints
- Add CPU profiling
- ScanBinary: check for ELF binary first
- ValidateGoLinux: remove
- Remove mime type check
- Make logging less verbose by default for `node_scan`

## [0.2.8] - 2023-06-22

### Features

- Add more binaries to filter list: `glibc_post_upgrade`, `ldconfig`, `sln`

## [0.2.7] - 2023-06-22

### Bug Fixes

- Cleanup and print operator components
- If there is no crypto then skip openssl test

## [0.2.6] - 2023-06-21

### Bug Fixes

- Add `dumb-init` to the filter list
- Fix missing header column

## [0.2.5] - 2023-06-21

### Bug Fixes

- Remove scanner and use buffer directly to prevent 'token too long' errors

## [0.2.4] - 2023-06-21

### Documentation

- Fix readme

### Fixes

- Fix `--verbose` option
- Fix parsing `--config`
- Fix bogus "found too many crypto libraries"

### Features

- Add `-u` and `-f` short commands
- Wire the klog flags
- Simplify logging
- Make logging less verbose by default
- Podman: simplify logging

## [0.2.3] - 2023-06-21

### Bug Fixes

- Fix libcrypto regex

## [0.2.2] - 2023-06-20

### Features

- Split filter-paths into dirs and files
- Scan for dependent openssl library within container

### Miscellaneous Tasks

- Go mod tidy

### Performance

- runNodeScan: Faster symlink detection
- scanBinary: Less repetitions
- scanBinary: pass topDir and innerPath

## [0.2.1] - 2023-06-20

### Documentation

- Update readme

## [0.2.0] - 2023-06-20

### Features

- Add cobra commandline control

## [0.1.9] - 2023-06-19

### Bug Fixes

- Should use 777 permissions

### Documentation

- Document filter for node scan
- Document pinns

### Features

- Add insecure pull option
- Add embedded ignore list and config file

## [0.1.8] - 2023-06-16

### Features

- Ignore removed file from node scan

## [0.1.7] - 2023-06-16

### Documentation

- Update readme

### Features

- Add multierror to capture all dependent binaries
- Add operator detection

## [0.1.6] - 2023-06-15

### Features

- Add build-locale-archive to the ignore list
- Check for `_cgo_init` (fixes 4.10)

## [0.1.5] - 2023-06-15

### Features

- Ignore `CGO_ENABLED` for golang <= 1.17 (fixes 4.10)

### Build

- Add latest to changelog generation

## [0.1.4] - 2023-06-15

### Documentation

- Add release information blurb

### Miscellaneous Tasks

- Use upstream golang image and remove port

### Performance

- Disable cgo... allows for slightly smaller binary

## [0.1.3] - 2023-06-14

### Miscellaneous Tasks

- first gitlab pipeline release

## [0.1.2] - 2023-06-14

### Miscellaneous Tasks

- Use git describe for version info

## [0.1.1] - 2023-06-14

### Documentation

- Add release and changelog

### Features

- Skip `CGO_ENABLED` check on go versions < 1.17
- Ignore `tini-static`
- Add golang tags validation

### Build

- Add Makefile

### Fixes

- Fix markdown lint
