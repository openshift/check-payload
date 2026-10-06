package inventory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// SourceOptions selects executable roots under an explicit build configuration.
type SourceOptions struct {
	Dir        string
	Patterns   []string
	BuildFlags []string
	Env        []string
	Tests      bool
	Progress   func(string)
}

// Source builds an SSA call graph rooted at each selected main and init.
// RTA is conservative and cannot prove FIPS-mode execution or guard correctness.
func Source(ctx context.Context, options SourceOptions) ([]Artifact, error) {
	progress := func(message string) {
		if options.Progress != nil {
			options.Progress(message)
		}
	}
	if len(options.Patterns) == 0 {
		return nil, fmt.Errorf("select executable packages explicitly")
	}
	config := &packages.Config{
		Context: ctx, Dir: options.Dir,
		Mode:       packages.LoadAllSyntax | packages.NeedModule,
		BuildFlags: options.BuildFlags, Tests: options.Tests,
		Env: append(os.Environ(), options.Env...),
	}
	progress("Loading selected executable packages and dependencies")
	pkgs, err := packages.Load(config, options.Patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading source: %w", err)
	}
	var loadErrors []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrors = append(loadErrors, e.Error())
		}
	})
	if len(loadErrors) > 0 {
		sort.Strings(loadErrors)
		return nil, fmt.Errorf("source loading incomplete:\n%s", strings.Join(loadErrors, "\n"))
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched the selected executable roots")
	}
	if !options.Tests {
		for _, p := range pkgs {
			if p.Name != "main" {
				return nil, fmt.Errorf("selected root %s is not executable", p.PkgPath)
			}
		}
	}
	prog, initial := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	progress("Building SSA for loaded packages")
	prog.Build()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	version := ""
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.Module != nil && p.Module.Path == "golang.org/x/crypto" {
			version = p.Module.Version
			if p.Module.Replace != nil {
				version = "replacement:" + p.Module.Replace.Path + "@" + p.Module.Replace.Version
			}
		}
	})
	settings, goVersion, err := sourceEnvironment(ctx, options)
	if err != nil {
		return nil, err
	}
	commit := sourceCommit(ctx, options.Dir)
	gitStatus := exec.CommandContext(ctx, "git", "status", "--porcelain")
	gitStatus.Dir = options.Dir
	if status, statusErr := gitStatus.Output(); statusErr == nil {
		settings["vcs.modified"] = fmt.Sprint(len(status) > 0)
	}
	artifacts := []Artifact{}
	for _, pkg := range initial {
		if pkg == nil || pkg.Pkg.Name() != "main" || pkg.Func("main") == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return artifacts, err
		}
		roots := []*ssa.Function{pkg.Func("main"), pkg.Func("init")}
		progress("Analyzing reachability for " + pkg.Pkg.Path())
		analysis := rta.Analyze(roots, true)
		if err := ctx.Err(); err != nil {
			return artifacts, err
		}
		progress(fmt.Sprintf("Extracting evidence from %d reachable functions for %s", len(analysis.Reachable), pkg.Pkg.Path()))
		paths := callPaths(analysis.CallGraph, roots)
		a := Artifact{
			Path: pkg.Pkg.Path(), GoVersion: goVersion, SourceCommit: commit,
			BuildSettings: settings, Coverage: "analyzed", Findings: []Finding{},
		}
		for fn := range analysis.Reachable {
			if fn.Pkg == nil {
				continue
			}
			pkgPath := fn.Pkg.Pkg.Path()
			if !strings.HasPrefix(pkgPath, xcrypto) && !strings.HasPrefix(pkgPath, "vendor/"+xcrypto) {
				continue
			}
			name := sourceSymbol(fn)
			chain := paths[fn]
			pkgVersion := version
			if strings.HasPrefix(pkgPath, "vendor/"+xcrypto) {
				pkgPath = strings.TrimPrefix(pkgPath, "vendor/")
				pkgVersion = "stdlib-vendor:" + goVersion
			}
			finding := classify(pkgPath, name, pkgVersion, chain)
			finding.Evidence = "source-reachable"
			finding.CallChain = chain
			if len(chain) == 0 {
				// Reflection-reachable methods need not have a graph edge.
				finding.Evidence = "source-reflection-candidate"
			}
			position := prog.Fset.Position(fn.Pos())
			if position.IsValid() {
				rel, err := filepath.Rel(options.Dir, position.Filename)
				if err == nil {
					finding.Source = fmt.Sprintf("%s:%d", filepath.ToSlash(rel), position.Line)
				}
			}
			a.Findings = append(a.Findings, finding)
		}
		artifacts = append(artifacts, a)
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("selected packages contain no executable main; use --tests for a shipped test executable")
	}
	return artifacts, nil
}

func sourceSymbol(fn *ssa.Function) string {
	if fn.Pkg == nil || fn.Signature.Recv() != nil {
		return fn.String()
	}
	return fn.Pkg.Pkg.Path() + "." + fn.Name()
}

func callPaths(graph *callgraph.Graph, roots []*ssa.Function) map[*ssa.Function][]string {
	paths := make(map[*ssa.Function][]string)
	// Receiver and generic function names are expensive to render. A large
	// reflection graph may compare the same function millions of times.
	sortNames := make(map[*ssa.Function]string)
	name := func(fn *ssa.Function) string {
		if value, ok := sortNames[fn]; ok {
			return value
		}
		value := fn.String()
		sortNames[fn] = value
		return value
	}
	queue := append([]*ssa.Function{}, roots...)
	for _, root := range roots {
		paths[root] = []string{sourceSymbol(root)}
	}
	for head := 0; head < len(queue); head++ {
		caller := queue[head]
		node := graph.Nodes[caller]
		if node == nil {
			continue
		}
		edges := append([]*callgraph.Edge{}, node.Out...)
		sort.Slice(edges, func(i, j int) bool { return name(edges[i].Callee.Func) < name(edges[j].Callee.Func) })
		for _, edge := range edges {
			callee := edge.Callee.Func
			if _, found := paths[callee]; found {
				continue
			}
			paths[callee] = append(append([]string{}, paths[caller]...), sourceSymbol(callee))
			queue = append(queue, callee)
		}
	}
	return paths
}

func sourceEnvironment(ctx context.Context, options SourceOptions) (map[string]string, string, error) {
	keys := []string{"GOVERSION", "GOOS", "GOARCH", "GOAMD64", "GOARM", "GOARM64", "CGO_ENABLED", "CC", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS", "GOFLAGS", "GOEXPERIMENT", "GOFIPS140"}
	cmd := exec.CommandContext(ctx, "go", append([]string{"env"}, keys...)...)
	cmd.Dir, cmd.Env = options.Dir, append(os.Environ(), options.Env...)
	data, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("reading source build environment: %w", err)
	}
	values := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(values) != len(keys) {
		return nil, "", fmt.Errorf("unexpected go env output")
	}
	settings := make(map[string]string)
	for i, key := range keys {
		settings[key] = values[i]
	}
	settings["analysisFlags"] = strings.Join(options.BuildFlags, " ")
	return settings, settings["GOVERSION"], nil
}

func sourceCommit(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = dir
	data, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
