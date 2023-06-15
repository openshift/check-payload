# Changelog

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

