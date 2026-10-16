package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/check-payload/internal/inventory"
	"github.com/openshift/check-payload/internal/podman"
	"github.com/openshift/check-payload/internal/scan"
	"github.com/openshift/check-payload/internal/types"
	"github.com/spf13/cobra"
)

func newInventoryCommand() *cobra.Command {
	var component, output, format string
	var timeout time.Duration
	cmd := &cobra.Command{Use: "inventory", Short: "Inventory x/crypto evidence without release-gating decisions"}
	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if format != "json" && format != "text" {
			return fmt.Errorf("unsupported report format %q", format)
		}
		if timeout <= 0 {
			return fmt.Errorf("time-limit must be positive")
		}
		return nil
	}
	cmd.PersistentFlags().StringVar(&component, "component", "", "Component identity for the report")
	cmd.PersistentFlags().StringVar(&output, "output-file", "", "Report file (default stdout)")
	cmd.PersistentFlags().StringVar(&format, "output-format", "json", "Report format: json or text")
	cmd.PersistentFlags().DurationVar(&timeout, "time-limit", 30*time.Minute, "Analysis deadline")
	write := func(report *inventory.Report) error {
		report.ScannerVersion = Commit
		if report.ScannerVersion == "" {
			report.ScannerVersion = "unknown"
		}
		for i := range report.Artifacts {
			if report.Artifacts[i].Component == "" {
				report.Artifacts[i].Component = report.Component
			}
		}
		out := os.Stdout
		if output != "" {
			var err error
			out, err = os.Create(output)
			if err != nil {
				return fmt.Errorf("opening inventory output: %w", err)
			}
			defer out.Close()
		}
		if err := report.Write(out, format); err != nil {
			return err
		}
		if report.Incomplete() {
			return fmt.Errorf("inventory coverage is incomplete; see the report")
		}
		return nil
	}
	var binaryPath string
	binary := &cobra.Command{
		Use: "binary", Short: "Inventory a built Go executable, including stripped and gzip executables",
		RunE: func(_ *cobra.Command, _ []string) error {
			report := inventory.NewReport("binary", component)
			report.Limitations = append(report.Limitations, "Runtime function tables omit fully inlined functions; linked functions may be runtime-guarded or non-security helpers.")
			report.Artifacts = append(report.Artifacts, inventory.Binary(binaryPath))
			return write(report)
		},
	}
	binary.Flags().StringVar(&binaryPath, "path", "", "Built executable path")
	_ = binary.MarkFlagRequired("path")
	cmd.AddCommand(binary)
	var sourceDir, goos, goarch, cgo string
	var patterns, buildFlags []string
	var includeTests bool
	source := &cobra.Command{
		Use: "source", Short: "Inventory potential crypto execution paths from selected executable source roots",
		RunE: func(command *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(command.Context(), timeout)
			defer cancel()
			dir, err := filepath.Abs(sourceDir)
			if err != nil {
				return err
			}
			options := inventory.SourceOptions{Dir: dir, Patterns: patterns, BuildFlags: buildFlags, Tests: includeTests}
			if cgo != "" && cgo != "0" && cgo != "1" {
				return fmt.Errorf("cgo-enabled must be 0 or 1")
			}
			options.Progress = func(message string) { fmt.Fprintln(os.Stderr, message) }
			for key, value := range map[string]string{"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": cgo} {
				if value != "" {
					options.Env = append(options.Env, key+"="+value)
				}
			}
			report := inventory.NewReport("source", component)
			report.Limitations = append(report.Limitations, "Rapid Type Analysis conservatively approximates reachability; reflection, unsafe, assembly, and runtime configuration need additional evidence.", "Source findings have not been reconciled with an image digest or a shipped binary; use the production build toolchain and flags.")
			report.Artifacts, err = inventory.Source(ctx, options)
			if err != nil {
				report.Artifacts = append(report.Artifacts, inventory.Artifact{Path: strings.Join(patterns, ","), Coverage: "failed", Errors: []string{err.Error()}, Findings: []inventory.Finding{}})
			}
			return write(report)
		},
	}
	source.Flags().StringVar(&sourceDir, "dir", ".", "Source checkout directory")
	source.Flags().StringSliceVar(&patterns, "packages", nil, "Executable package roots, comma-separated (not dependency-presence scanning)")
	source.Flags().StringArrayVar(&buildFlags, "build-flag", nil, "Go loading/build flag, repeatable (e.g. --build-flag=-mod=vendor)")
	source.Flags().StringVar(&goos, "goos", "", "Production target OS (default Go environment)")
	source.Flags().StringVar(&goarch, "goarch", "", "Production target architecture (default Go environment)")
	source.Flags().StringVar(&cgo, "cgo-enabled", "", "Production CGO_ENABLED (0 or 1)")
	source.Flags().BoolVar(&includeTests, "tests", false, "Include a shipped test executable's generated main")
	_ = source.MarkFlagRequired("packages")
	cmd.AddCommand(source)
	var root, localImage string
	var localExpected []string
	local := &cobra.Command{
		Use: "local", Short: "Inventory Go executables in an unpacked image rootfs",
		RunE: func(command *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(command.Context(), timeout)
			defer cancel()
			report := inventory.NewReport("local", component)
			report.Limitations = append(report.Limitations, "Linked runtime function metadata is incomplete for fully inlined functions; symlinks are not followed.")
			report.Artifacts = inventory.Local(ctx, root, localImage)
			report.Artifacts = inventory.RequirePaths(report.Artifacts, localExpected, localImage)
			return write(report)
		},
	}
	local.Flags().StringVar(&root, "path", "", "Unpacked image rootfs directory")
	local.Flags().StringVar(&localImage, "image", "", "Provenance image digest/pullspec")
	local.Flags().StringSliceVar(&localExpected, "expected-paths", nil, "Expected Go executable paths within the image, comma-separated")
	_ = local.MarkFlagRequired("path")
	cmd.AddCommand(local)
	var image, pullSecret string
	var imageExpected []string
	imageCmd := &cobra.Command{
		Use: "image", Short: "Inventory an image using the existing Podman extraction path",
		RunE: func(command *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(command.Context(), timeout)
			defer cancel()
			report := inventory.NewReport("image", component)
			report.Artifacts = inventoryImage(ctx, image, pullSecret)
			report.Artifacts = inventory.RequirePaths(report.Artifacts, imageExpected, image)
			return write(report)
		},
	}
	imageCmd.Flags().StringVar(&image, "spec", "", "Image pullspec (prefer a digest)")
	imageCmd.Flags().StringVar(&pullSecret, "pull-secret", "", "Registry authentication file")
	imageCmd.Flags().StringSliceVar(&imageExpected, "expected-paths", nil, "Expected Go executable paths within the image, comma-separated")
	_ = imageCmd.MarkFlagRequired("spec")
	cmd.AddCommand(imageCmd)
	var payload, payloadSecret string
	var components []string
	payloadCmd := &cobra.Command{
		Use: "payload", Short: "Roll up Go inventories across selected payload images",
		RunE: func(command *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(command.Context(), timeout)
			defer cancel()
			report := inventory.NewReport("payload", component)
			info, err := scan.GetPayload(&types.Config{FromURL: payload, PullSecret: payloadSecret})
			if err != nil {
				report.Artifacts = []inventory.Artifact{{Path: payload, Image: payload, Coverage: "failed", Errors: []string{err.Error()}, Findings: []inventory.Finding{}}}
				return write(report)
			}
			found := map[string]bool{}
			for _, tag := range info.References.Spec.Tags {
				selected := len(components) == 0
				for _, name := range components {
					selected = selected || tag.Name == name
				}
				if !selected || tag.From == nil {
					continue
				}
				found[tag.Name] = true
				artifacts := inventoryImage(ctx, tag.From.Name, payloadSecret)
				for i := range artifacts {
					artifacts[i].Component = tag.Name
					if artifacts[i].BuildSettings == nil {
						artifacts[i].BuildSettings = make(map[string]string)
					}
					artifacts[i].BuildSettings["payloadTag"] = tag.Name
				}
				report.Artifacts = append(report.Artifacts, artifacts...)
			}
			for _, name := range components {
				if !found[name] {
					report.Artifacts = append(report.Artifacts, inventory.Artifact{Path: name, Coverage: "failed", Errors: []string{"requested component missing from payload"}, Findings: []inventory.Finding{}})
				}
			}
			return write(report)
		},
	}
	payloadCmd.Flags().StringVar(&payload, "url", "", "Release payload pullspec")
	payloadCmd.Flags().StringVar(&payloadSecret, "pull-secret", "", "Registry authentication file")
	payloadCmd.Flags().StringSliceVar(&components, "components", nil, "Payload image tags to inventory")
	_ = payloadCmd.MarkFlagRequired("url")
	cmd.AddCommand(payloadCmd)
	return cmd
}

func inventoryImage(ctx context.Context, spec, secret string) []inventory.Artifact {
	fail := func(err error) []inventory.Artifact {
		return []inventory.Artifact{{Path: spec, Image: spec, Coverage: "failed", Errors: []string{err.Error()}, Findings: []inventory.Finding{}}}
	}
	if err := podman.PullWithAuth(ctx, spec, secret); err != nil {
		return fail(err)
	}
	identity, err := podman.Inspect(ctx, spec, "--format", "{{.Id}}")
	if err != nil {
		return fail(err)
	}
	identity = strings.TrimSpace(identity)
	root, err := podman.Mount(ctx, identity)
	if err != nil {
		return fail(err)
	}
	imageReference := spec
	if !strings.Contains(spec, "@sha256:") {
		if data, inspectErr := podman.Inspect(ctx, spec, "--format", "{{json .RepoDigests}}"); inspectErr == nil {
			var digests []string
			if json.Unmarshal([]byte(data), &digests) == nil && len(digests) > 0 {
				imageReference = digests[0]
			}
		}
	}
	artifacts := inventory.Local(ctx, root, imageReference)
	imageSourceCommit, _ := podman.Inspect(ctx, spec, "--format", "{{index .Labels \"io.openshift.build.commit.id\"}}")
	imageSourceCommit = strings.TrimSpace(imageSourceCommit)
	for i := range artifacts {
		if artifacts[i].BuildSettings == nil {
			artifacts[i].BuildSettings = make(map[string]string)
		}
		artifacts[i].BuildSettings["imageConfigID"] = identity
		if imageSourceCommit != "" && imageSourceCommit != "<no value>" {
			artifacts[i].BuildSettings["imageSourceCommit"] = imageSourceCommit
		}
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
	defer cancel()
	if err := podman.Unmount(cleanupCtx, identity); err != nil {
		artifacts = append(artifacts, fail(err)...)
	}
	return artifacts
}
