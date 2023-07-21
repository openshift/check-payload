package scan

import (
	"fmt"
	"os"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/jedib0t/go-pretty/v6/table"
	"k8s.io/klog/v2"

	"github.com/openshift/check-payload/internal/types"
)

var (
	colTitleOperatorName = "Operator Name"
	colTitleTagName      = "Tag Name"
	colTitleRPMName      = "RPM Name"
	colTitleExeName      = "Executable Name"
	colTitlePassedFailed = "Status"
	colTitleImage        = "Image"
	failureRowHeader     = table.Row{colTitleOperatorName, colTitleTagName, colTitleRPMName, colTitleExeName, colTitlePassedFailed, colTitleImage}
	successRowHeader     = table.Row{colTitleOperatorName, colTitleTagName, colTitleExeName, colTitleImage}
)

var (
	colTitleNodePath         = "Path"
	colTitleNodePassedFailed = "Status"
	colTitleNodeFrom         = "From"
	failureNodeRowHeader     = table.Row{colTitleNodePath, colTitleNodePassedFailed, colTitleNodeFrom}
	successNodeRowHeader     = table.Row{colTitleNodePath}
)

func PrintResults(cfg *types.Config, results []*types.ScanResults) {
	var failureReport, warningReport, successReport string

	var combinedReport string

	if cfg.NodeScan != "" {
		failureReport, warningReport, successReport = generateNodeScanReport(results, cfg)
	} else {
		failureReport, warningReport, successReport = generateReport(results, cfg)
	}

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
	if res.Component != nil && res.Component.Component != "" {
		return "payload." + res.Component.Component
	}
	if res.Tag != nil && res.Tag.Name != "" {
		return "tag." + res.Tag.Name
	}
	return ""
}

func displayExceptions(results []*types.ScanResults) {
	exceptions := make(map[string]mapset.Set[string])
	for _, result := range results {
		for _, res := range result.Items {
			if res.Error == nil || res.Path == "" {
				// Skip over successes and errors with no path set.
				continue
			}
			prefix := getFilterPrefix(res)
			if set, ok := exceptions[prefix]; ok {
				set.Add(res.Path)
			} else {
				exceptions[prefix] = mapset.NewSet(res.Path)
			}
		}
	}

	for prefix, set := range exceptions {
		if prefix != "" {
			fmt.Printf("[%s]\n", prefix)
		}
		ss := set.ToSlice()
		if len(ss) == 1 {
			fmt.Printf("filter_files = [ %q ]\n", ss[0])
		} else {
			fmt.Println("filter_files = [")
			for _, res := range ss {
				fmt.Printf("  %q,\n", res)
			}
			fmt.Println("]")
		}
		fmt.Println("")
	}
}

func generateNodeScanReport(results []*types.ScanResults, cfg *types.Config) (string, string, string) {
	var failureTableRows []table.Row
	var warningTableRows []table.Row
	var successTableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			if res.IsLevel(types.Error) {
				failureTableRows = append(failureTableRows, table.Row{res.Path, res.Error.GetError(), res.RPM})
			} else if res.IsLevel(types.Warning) {
				warningTableRows = append(warningTableRows, table.Row{res.Path, res.Error.GetError(), res.RPM})
			} else {
				successTableRows = append(successTableRows, table.Row{res.Path})
			}
		}
	}

	ftw := table.NewWriter()
	ftw.AppendHeader(failureNodeRowHeader)
	ftw.AppendRows(failureTableRows)
	ftw.SetIndexColumn(1)

	wtw := table.NewWriter()
	wtw.AppendHeader(failureNodeRowHeader)
	wtw.AppendRows(warningTableRows)
	wtw.SetIndexColumn(1)

	stw := table.NewWriter()
	stw.AppendHeader(successNodeRowHeader)
	stw.AppendRows(successTableRows)
	stw.SetIndexColumn(1)

	return generateOutputString(cfg, ftw, wtw, stw)
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
	return "<unknown>"
}

func renderReport(results []*types.ScanResults) (failures table.Writer, warnings table.Writer, successes table.Writer) {
	var failureTableRows []table.Row
	var warningTableRows []table.Row
	var successTableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			component := getComponent(res)
			if res.IsLevel(types.Error) {
				failureTableRows = append(failureTableRows, table.Row{component, res.Tag.Name, res.RPM, res.Path, res.Error.GetError(), res.Tag.From.Name})
			} else if res.IsLevel(types.Warning) {
				warningTableRows = append(warningTableRows, table.Row{component, res.Tag.Name, res.RPM, res.Path, res.Error.GetError(), res.Tag.From.Name})
			} else {
				successTableRows = append(successTableRows, table.Row{component, res.Tag.Name, res.Path, res.Tag.From.Name})
			}
		}
	}

	ftw := table.NewWriter()
	ftw.AppendHeader(failureRowHeader)
	ftw.AppendRows(failureTableRows)
	ftw.SetIndexColumn(1)

	wtw := table.NewWriter()
	wtw.AppendHeader(failureRowHeader)
	wtw.AppendRows(warningTableRows)
	wtw.SetIndexColumn(1)

	stw := table.NewWriter()
	stw.AppendHeader(successRowHeader)
	stw.AppendRows(successTableRows)
	stw.SetIndexColumn(1)
	return ftw, wtw, stw
}
