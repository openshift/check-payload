package main

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"k8s.io/klog/v2"

	mapset "github.com/deckarep/golang-set/v2"
)

var (
	colTitleOperatorName = "Operator Name"
	colTitleTagName      = "Tag Name"
	colTitleExeName      = "Executable Name"
	colTitlePassedFailed = "Status"
	colTitleImage        = "Image"
	failureRowHeader     = table.Row{colTitleOperatorName, colTitleTagName, colTitleExeName, colTitlePassedFailed, colTitleImage}
	successRowHeader     = table.Row{colTitleOperatorName, colTitleTagName, colTitleExeName, colTitleImage}
)

var (
	colTitleNodePath         = "Path"
	colTitleNodePassedFailed = "Status"
	colTitleNodeFrom         = "From"
	failureNodeRowHeader     = table.Row{colTitleNodePath, colTitleNodePassedFailed, colTitleNodeFrom}
	successNodeRowHeader     = table.Row{colTitleNodePath}
)

func printResults(cfg *Config, results []*ScanResults) {
	var failureReport, warningReport, successReport string

	var combinedReport string

	if cfg.NodeScan != "" {
		failureReport, warningReport, successReport = generateNodeScanReport(results, cfg)
	} else {
		failureReport, warningReport, successReport = generateReport(results, cfg)
	}

	isWarnings := isWarnings(results)
	isFailed := isFailed(results)
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

	if !isFailed && (!cfg.FailOnWarnings && isWarnings) {
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

func displayExceptions(results []*ScanResults) {
	exceptions := make(map[string]mapset.Set[*ScanResult])
	for _, result := range results {
		for _, res := range result.Items {
			if res.Error == nil {
				// skip over successes
				continue
			}
			component := getComponent(res)
			if set, ok := exceptions[component.Component]; ok {
				set.Add(res)
			} else {
				exceptions[component.Component] = mapset.NewSet(res)
			}
		}
	}

	for payloadName, set := range exceptions {
		if payloadName != "" {
			fmt.Printf("[payload.%v]\n", payloadName)
		}
		if len(set.ToSlice()) == 1 {
			fmt.Printf("filter_files = [ \"%v\" ]\n", set.ToSlice()[0].Path)
		} else {
			fmt.Printf("filter_files = [\n")
			for _, res := range set.ToSlice() {
				fmt.Printf("  \"%v\",\n", res.Path)
			}
			fmt.Printf("]\n")
		}
		fmt.Println("")
	}
}

func generateNodeScanReport(results []*ScanResults, cfg *Config) (string, string, string) {
	var failureTableRows []table.Row
	var warningTableRows []table.Row
	var successTableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			if res.IsLevel(Error) {
				failureTableRows = append(failureTableRows, table.Row{res.Path, res.Error.GetError(), res.Tag.From.Name})
			} else if res.IsLevel(Warning) {
				warningTableRows = append(warningTableRows, table.Row{res.Path, res.Error.GetError(), res.Tag.From.Name})
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

func generateReport(results []*ScanResults, cfg *Config) (string, string, string) {
	ftw, wtw, stw := renderReport(results)
	return generateOutputString(cfg, ftw, wtw, stw)
}

func generateOutputString(cfg *Config, ftw table.Writer, wtw table.Writer, stw table.Writer) (string, string, string) {
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

func getComponent(res *ScanResult) *OpenshiftComponent {
	if res.Component != nil {
		return res.Component
	}
	return &OpenshiftComponent{
		Component: "<unknown>",
	}
}

func renderReport(results []*ScanResults) (failures table.Writer, warnings table.Writer, successes table.Writer) {
	var failureTableRows []table.Row
	var warningTableRows []table.Row
	var successTableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			component := getComponent(res)
			if res.IsLevel(Error) {
				failureTableRows = append(failureTableRows, table.Row{component.Component, res.Tag.Name, res.Path, res.Error.GetError(), res.Tag.From.Name})
			} else if res.IsLevel(Warning) {
				warningTableRows = append(warningTableRows, table.Row{component.Component, res.Tag.Name, res.Path, res.Error.GetError(), res.Tag.From.Name})
			} else {
				successTableRows = append(successTableRows, table.Row{component.Component, res.Tag.Name, res.Path, res.Tag.From.Name})
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
