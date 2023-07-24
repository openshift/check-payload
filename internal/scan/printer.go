package scan

import (
	"fmt"
	"os"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/jedib0t/go-pretty/v6/table"
	"k8s.io/klog/v2"

	"github.com/openshift/check-payload/internal/types"
)

const (
	colTitleOperatorName = "Operator Name"
	colTitleTagName      = "Tag Name"
	colTitleRPMName      = "RPM Name"
	colTitleExeName      = "Executable Name"
	colTitlePassedFailed = "Status"
	colTitleImage        = "Image"
)

func PrintResults(cfg *types.Config, results []*types.ScanResults) {
	var failureReport, warningReport, successReport string

	var combinedReport string

	failureReport, warningReport, successReport = generateReport(results, cfg)

	isWarnings := IsWarnings(results)
	isFailed := IsFailed(results)
	if isFailed {
		fmt.Println("---- Failure Report")
		fmt.Println(failureReport)
		combinedReport = failureReport
	}

	if isWarnings {
		fmt.Println("---- Warning Report")
		fmt.Println(warningReport)
		combinedReport += "\n\n ---- Warning Report\n" + warningReport
	}

	if cfg.Verbose {
		fmt.Println("---- Success Report")
		fmt.Println(successReport)
		combinedReport += "\n\n ---- Success Report\n" + successReport
	}

	if !isFailed && isWarnings {
		combinedReport += "\n\n ---- Successful run with warnings\n"
		fmt.Println("---- Successful run with warnings")
	}

	if !isFailed && !isWarnings {
		combinedReport += "\n\n ---- Successful run\n"
		fmt.Println("---- Successful run")
	}

	if cfg.OutputFile != "" {
		if err := os.WriteFile(cfg.OutputFile, []byte(combinedReport), 0o777); err != nil {
			klog.Errorf("could not write file: %v", err)
		}
	}

	if cfg.PrintExceptions {
		displayExceptions(results)
	}
}

func getFilterPrefix(res *types.ScanResult) string {
	if res.RPM != "" {
		return "rpm." + res.RPM
	}
	if component := getComponent(res); component != "" {
		return "payload." + component
	}
	if tag := getTag(res); tag != "" {
		return "tag." + tag
	}
	return ""
}

func displayExceptions(results []*types.ScanResults) {
	// Per-prefix map of per-error map of files to be excluded.
	exceptions := make(map[string]map[string]mapset.Set[string])
	for _, result := range results {
		for _, res := range result.Items {
			if res.Error == nil || res.Path == "" {
				// Skip over successes and errors with no path set.
				continue
			}
			prefix := getFilterPrefix(res)
			errMap, ok := exceptions[prefix]
			if !ok {
				errMap = make(map[string]mapset.Set[string])
				exceptions[prefix] = errMap
			}

			errName := types.KnownErrorName(res.Error.Error)
			if set, ok := errMap[errName]; ok {
				set.Add(res.Path)
			} else {
				errMap[errName] = mapset.NewSet(res.Path)
			}
		}
	}

	for prefix, errMap := range exceptions {
		for errName, set := range errMap {
			if prefix != "" {
				fmt.Printf("[[%s.ignore]]\n", prefix)
			} else {
				fmt.Println("[[ignore]]")
			}
			fmt.Printf("error = %q\n", errName)
			ss := set.ToSlice()
			if len(ss) == 1 {
				fmt.Printf("files = [ %q ]\n", ss[0])
			} else {
				fmt.Println("files = [")
				sort.Strings(ss)
				for _, res := range ss {
					fmt.Printf("  %q,\n", res)
				}
				fmt.Println("]")
			}
			fmt.Println("")
		}
	}
}

func generateReport(results []*types.ScanResults, cfg *types.Config) (string, string, string) {
	ftw, wtw, stw := renderReport(results)
	return generateOutputString(cfg, ftw, wtw, stw)
}

func generateOutputString(cfg *types.Config, ftw table.Writer, wtw table.Writer, stw table.Writer) (string, string, string) {
	var failureReport string
	var successReport string
	var warningReport string

	switch cfg.OutputFormat {
	case "table":
		failureReport = ftw.Render()
		successReport = stw.Render()
		warningReport = wtw.Render()
	case "csv":
		failureReport = ftw.RenderCSV()
		successReport = stw.RenderCSV()
		warningReport = wtw.RenderCSV()
	case "markdown":
		failureReport = ftw.RenderMarkdown()
		successReport = stw.RenderMarkdown()
		warningReport = wtw.RenderMarkdown()
	case "html":
		failureReport = ftw.RenderHTML()
		successReport = stw.RenderHTML()
		warningReport = wtw.RenderHTML()
	}

	return failureReport, warningReport, successReport
}

func getComponent(res *types.ScanResult) string {
	if res.Component != nil {
		return res.Component.Component
	}
	return ""
}

func getTag(res *types.ScanResult) string {
	if res.Tag != nil {
		return res.Tag.Name
	}
	return ""
}

func getImage(res *types.ScanResult) string {
	if res.Tag != nil && res.Tag.From != nil {
		return res.Tag.From.Name
	}
	return ""
}

func renderReport(results []*types.ScanResults) (failures table.Writer, warnings table.Writer, successes table.Writer) {
	var failureTableRows, warningTableRows, successTableRows []table.Row

	failureRowHeader := table.Row{colTitleOperatorName, colTitleTagName, colTitleRPMName, colTitleExeName, colTitlePassedFailed, colTitleImage}
	successRowHeader := table.Row{colTitleOperatorName, colTitleTagName, colTitleExeName, colTitleImage}

	for _, result := range results {
		for _, res := range result.Items {
			component := getComponent(res)
			tag := getTag(res)
			image := getImage(res)

			if res.IsLevel(types.Error) {
				failureTableRows = append(failureTableRows, table.Row{component, tag, res.RPM, res.Path, res.Error.GetError(), image})
			} else if res.IsLevel(types.Warning) {
				warningTableRows = append(warningTableRows, table.Row{component, tag, res.RPM, res.Path, res.Error.GetError(), image})
			} else {
				successTableRows = append(successTableRows, table.Row{component, tag, res.Path, image})
			}
		}
	}

	ftw := table.NewWriter()
	ftw.SuppressEmptyColumns()
	ftw.AppendHeader(failureRowHeader)
	ftw.AppendRows(failureTableRows)
	ftw.SetIndexColumn(1)

	wtw := table.NewWriter()
	wtw.SuppressEmptyColumns()
	wtw.AppendHeader(failureRowHeader)
	wtw.AppendRows(warningTableRows)
	wtw.SetIndexColumn(1)

	stw := table.NewWriter()
	stw.SuppressEmptyColumns()
	stw.AppendHeader(successRowHeader)
	stw.AppendRows(successTableRows)
	stw.SetIndexColumn(1)
	return ftw, wtw, stw
}
