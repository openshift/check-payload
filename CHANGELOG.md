# Changelog

## [2.0.14] - 2023-06-26

### Bug Fixes

- Use file open instead of strings

## [2.0.13] - 2023-06-26

### Bug Fixes

- Fix openssl detection
- Check for fips_mode

### Features

- Add more binaries to filter list: grpc_health_probe
- 2.0.10 release
- 2.0.11 release
- 2.0.12 release
- Add /usr/local/bin/catatonit
- Add sysroot ignore
- Use rootfs

## [2.0.12] - 2023-06-23

### Bug Fixes

- Fix openssl detection

## [2.0.11] - 2023-06-23

### Features

- Add more binaries to filter list: grpc_health_probe
- Add support for supplying pull spec config file

## [2.0.11] - 2023-06-23

### Features

- Add more binaries to filter list: grpc_health_probe
- 2.0.10 release

## [2.0.10] - 2023-06-22

### Features

- Add more binaries to filter list: grpc_health_probe

## [2.0.9] - 2023-06-22

### Bug Fixes

- Cleanup container

### Features

- Use backup entrypoint /bin/sh
- Allow for alternate entrypoints
- Add CPU profiling

### ScanBinary

- Check for ELF binary first

### ValidateGoLinux

- Remove

## [2.0.8] - 2023-06-22

### Features

- Add more binaries to filter list: glibc_post_upgrade, ldconfig, sln

## [2.0.7] - 2023-06-22

### Bug Fixes

- Cleanup and print operator components
- If there is no crypto then skip openssl test

## [2.0.6] - 2023-06-21

### Bug Fixes

- Add dumb-init to the filter list

### Printer

- Fix missing header column

## [2.0.5] - 2023-06-21

### Bug Fixes

- Remove scanner and use buffer directly to prevent 'token too long' errors

## [2.0.4] - 2023-06-21

### Bug Fixes

- Fix verbose option

### Documentation

- Fix readme

### Features

- Add -u and -f short commands

### Podman

- Simplify logging

## [2.0.3] - 2023-06-21

### Bug Fixes

- Fix libcrypto regex

## [2.0.2] - 2023-06-20

### Features

- Split filter-paths into dirs and files
- Scan for dependent openssl library within container

### Miscellaneous Tasks

- Go mod tidy

### RunNodeScan

- Faster symlink detection

### ScanBinary

- Less repetitions
- Pass topDir and innerPath

## [2.0.0] - 2023-06-20

### Features

- Add cobra commandline control

## [1.0.9] - 2023-06-19

### Bug Fixes

- Should use 777 permissions

### Documentation

- Document filter for node scan
- Document pinns

### Features

- Add insecure pull option
- Add embedded ignore list and config file

## [1.0.8] - 2023-06-16

### Features

- Ignore removed file from node scan

## [1.0.7] - 2023-06-16

### Documentation

- Update readme

### Features

- Add multierror to capture all dependent binaries
- Add operator detection

## [1.0.6] - 2023-06-15

### Documentation

- Add build-locale-archive to the ignore list

### Features

- Check for _cgo_init (fixes 4.10)

## [1.0.5] - 2023-06-15

### Features

- Ignore CGO_ENABLED for golang <= 1.17 (fixes 4.10)

### Build

- Add latest to changelog generation

## [1.0.4] - 2023-06-15

### Documentation

- Add release information blurb

### Miscellaneous Tasks

- Use upstream golang image and remove port

### Performance

- Disable cgo... allows for slightly smaller binary

## [1.0.3] - 2023-06-14

### Miscellaneous Tasks

- first gitlab pipeline release

## [1.0.2] - 2023-06-14

### Miscellaneous Tasks

- Use git describe for version info
- Artifact build only on tags
- Update .gitlab-ci.yml file
- Use git describe for version info

## [1.0.1] - 2023-06-14

### Features

- Skip CGO_ENABLED check on go versions < 1.17
- Ignore tini-static
- Add golang tags validation

