# Retained Go crypto target detection

Use `scan --detect-go-crypto` to look for configured function/package identities
in shipped Go executables. This supplements source/dependency scanners: it can
show that a flagged function survived linking. It does not perform source
analysis, construct a CBOM or certify FIPS compliance.

```sh
check-payload scan --detect-go-crypto image --spec IMAGE_PULL_SPEC
check-payload scan --detect-go-crypto payload --spec RELEASE_PULL_SPEC
check-payload scan --detect-go-crypto local --path UNPACKED_IMAGE
```

The detector runs in the existing directory-walk scanner for image, payload,
local and `node --walk-scan` inputs. It is disabled by default. RPM-only scanning
is not supported with this flag. The existing FIPS build checks still run and
retain their existing exit behavior.

## Configuration

Targets live in the existing TOML configuration and can be selected with
`--config` and supplemented with `--config-for-version` like other settings.
The initial list is deliberately small and requires review before rollout; it
is not a complete list of problematic crypto.

```toml
[[go_crypto_targets]]
package = "golang.org/x/crypto/sha3"
symbol = "golang.org/x/crypto/sha3.NewLegacyKeccak256"
reason = "Legacy Keccak requires review of security use."
reference = "https://redhat.atlassian.net/browse/DPTP-5284"
```

`package` matches exactly. Optional `symbol` matches an exact full runtime
function name, including any receiver or compiler suffix. Omitting it matches
all retained functions in the package, including initialization; a package
match does not establish primitive execution. Subpackages do not match a parent
package selector. There are no regexes, glob patterns, version predicates or
classification rules. `reason` is required; `reference` is optional. For
example, the same selectors work for `example.org/primitive`; extraction has no
hardcoded x/crypto filter.

Only a leading `vendor/` prefix is normalized for matching standard-library
vendored packages. Output preserves the full original symbol to distinguish it
from a module dependency. If targets overlap, the first matching explanation is
reported for each function.

## Results and coverage

Matches appear as ordinary scanner warning rows with binary path, image/tag,
component, retained symbol and configured explanation/reference. Successfully
parsed Go executables with no matches get a success row. Failures to read Go
metadata are warnings describing incomplete detection. Unrelated files are
skipped. Detection is independent of FIPS build validation, so it still reports
matches when a separate build check fails.

Matches are review evidence rather than failures by default. Existing
`--fail-on-warnings` makes these warnings fail the scan too. Do not enable that
flag for a report-only pilot. Other existing validation failures and traversal
errors can still fail the command.

Ordinary stripped Go ELF binaries are supported through runtime function tables;
debug symbol tables are unnecessary. Go Mach-O executables and gzip-transported
executables are also examined without requiring execute permission. Gzip
expansion is limited to 512 MiB per file. Traversal does not follow symlinks.
The scanner honors existing file/directory/component filters. In particular,
the default `/usr/share/openshift` directory filter excludes installer artifacts:
use a reviewed `--config` file that omits that filter when collecting them.
RPM-only scans, Windows executables, obfuscated binaries and formats without
readable Go metadata are outside this detector's coverage.

A retained function is not proof it executes, is used for security, runs in
FIPS mode or lacks a runtime guard. A no-match result is not proof of absence:
fully inlined code may have no separate retained function, metadata can be
removed, selectors can be incomplete, and filters may exclude artifacts. This
detector does not infer algorithm negotiation, exceptions or compliance classes.
