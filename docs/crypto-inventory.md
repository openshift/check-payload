# Crypto evidence inventory

`check-payload inventory` records x/crypto evidence independently of the existing
FIPS build checks. Crypto findings do not change the exit status. Incomplete
coverage does return a failure, with a report where possible, so missing evidence
cannot silently become a clean inventory.

## Commands

```sh
check-payload inventory source --dir . --packages ./cmd/server,./cmd/helper \
  --goos linux --goarch amd64 --cgo-enabled 1 \
  --build-flag=-mod=vendor --build-flag=-tags=production \
  --component example --output-file source.json
check-payload inventory binary --path ./server --output-file binary.json
check-payload inventory local --path ./rootfs \
  --image registry.example/component@sha256:... --output-file image.json
check-payload inventory image --spec registry.example/component@sha256:... \
  --pull-secret ./auth.json --output-file image.json
check-payload inventory payload --url registry.example/release@sha256:... \
  --components hyperkube,kube-proxy --pull-secret ./auth.json \
  --output-file payload.json
```

Use `--output-format text` for a readable view of the same findings. Image and
payload modes use Podman (use `podman unshare check-payload inventory image ...`
for rootless mounts); local mode accepts an already extracted image. Always
retain the immutable pullspec used for extraction. Local mode does not discover
an image identity or image labels independently.

Source mode requires explicit executable roots. Use `--tests` only for test
executables that are shipped in an image. It loads the selected build configuration,
builds SSA, and applies Rapid Type Analysis rooted at each executable's main and
initialization functions. A call chain is one possible path, not an exhaustive
list of callers. Reflection candidates without a path are identified separately.
Imports, unused dependency functions, and go.mod entries alone are not findings.

Binary mode reads Go runtime function metadata, including ordinary `-s -w`
stripped ELF and Mach-O executables, and gzip-transported executables. Fully
inlined functions can be absent. A linked function is not proof of execution;
use source and runtime evidence to investigate its context. Missing or damaged
metadata is a coverage error. Local mode finds regular files by their format,
including non-executable transported binaries; it does not follow symlinks.

Both modes include `vendor/golang.org/x/crypto` from the Go standard library and
identify that origin separately from a module dependency. Binary build settings
include the executable main package and main module, so inherited base-image
binaries can be mapped to their own source enrollment instead of being attributed
to the repository that built the outer image. Forks and old wrapper
versions do not inherit a modern delegation classification automatically.

## Interpretation

The embedded policy is **provisional**, derived from OCPSTRAT-3446 and pending
review against the authoritative policy document. F1/F2/F3 are candidate labels.
Context-sensitive packages remain unresolved, including Blowfish when one source
path passes through bcrypt. That path does not prove all uses are substrate uses.

Guard correctness, security intent, exception ownership/expiry, P1 boundaries,
and actual ML-KEM negotiation require additional evidence. Guard status is always
unresolved in this prototype. This command grants no exceptions and makes no
release-blocking decisions.

Source analysis must run with the production compiler, build flags, target
platform, CGO settings, and source revision. Reports record those inputs and dirty
tree state. Independently acquired source and image reports must be reconciled
using the image's build metadata before claiming they describe the same build.
ART image source labels may identify an upstream revision while embedded VCS
metadata identifies a patched build tree. Preserve both; do not substitute one
for the other. Compare embedded build settings too: downstream builds can
override CGO, architecture feature levels or tags from the repository recipe.
Assembly, unsafe, native libraries, plugins, dynamic loading, and reflection
prevent a complete execution proof. Only x/crypto functions are inventoried.

Source analysis can require several GiB of memory. Run large components
sequentially and allocate a dedicated CI worker. For very large repositories,
invoke one executable root per process to avoid retaining unrelated SSA graphs. `--time-limit` cancels package
loading and is checked between SSA/RTA phases; those graph phases are not
preemptible. Set a job-level process timeout and memory limit as well.

## Periodic collection

Run a pinned scanner in the component's production build root. Keep source and
binary reports separate, then publish them together under
`$ARTIFACT_DIR/crypto-inventory/<component>/<build-id>/`. Record the source commit,
image digest, selected executable roots/platforms and scanner/policy versions in
the job manifest. Pass `--expected-paths /usr/bin/server,/usr/bin/helper.gz` to image or local
mode to record missing expected binaries as coverage failures. Rootfs discovery
alone cannot prove an expected executable is present.

Validate each JSON report with
[report.schema.json](../internal/inventory/report.schema.json), for example using
a pinned Python jsonschema environment:

```sh
python3 hack/validate-crypto-inventory.py source.json image.json
```

This validation checks the report contract. Coverage failures still belong in
published artifacts. The collector must preserve the scanner's failure status
rather than treating a schema-valid error report as successful analysis.

Build a central index with `python3 hack/index-crypto-inventory.py
--artifact-root "$ARTIFACT_DIR" --output "$ARTIFACT_DIR/index.json" <reports...>`.
Use repeatable `--expected-report <path>` arguments from the job manifest so
missing reports remain visible and the index is marked incomplete. It
schema-validates each report, links artifact paths, records content hashes and
summarizes coverage and candidate classifications per executable. Multiple copies
of an executable remain separate inventory entries. Publish the index and reports
together at an immutable per-build location. Index creation does not reconcile
source and binary findings or grant policy approval. Detection rollout is separate from a later
release-gating and exception-register workflow. The command is CI-system
independent; Prow, ART, layered builds and optional Konflux adapters can invoke
the same scanner. `build-machinery-go` can supply an opt-in source invocation.
