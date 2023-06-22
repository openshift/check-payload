package main

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"k8s.io/klog/v2"
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
	var failureReport, successReport string

	var combinedReport string

	if cfg.NodeScan != "" {
		failureReport, successReport = generateNodeScanReport(results, cfg)
	} else {
		failureReport, successReport = generateReport(results, cfg)
	}

	failed := isFailed(results)
	if failed {
		fmt.Println("---- Failure Report")
		fmt.Println(failureReport)
		combinedReport = failureReport
	}

	if cfg.Verbose {
		fmt.Println("---- Success Report")
		fmt.Println(successReport)
		combinedReport += "\n\n ---- Success Report\n" + successReport
	}

	if !failed {
		combinedReport += "\n\n ---- Successful run\n"
		fmt.Println("---- Successful run")
	}

	if cfg.OutputFile != "" {
		if err := os.WriteFile(cfg.OutputFile, []byte(combinedReport), 0o777); err != nil {
			klog.Errorf("could not write file: %v", err)
		}
	}
}

func generateNodeScanReport(results []*ScanResults, cfg *Config) (string, string) {
	var failureTableRows []table.Row
	var successTableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			if res.Error != nil {
				failureTableRows = append(failureTableRows, table.Row{res.Path, res.Error, res.Tag.From.Name})
			} else {
				successTableRows = append(successTableRows, table.Row{res.Path})
			}
		}
	}

	ftw := table.NewWriter()
	ftw.AppendHeader(failureNodeRowHeader)
	ftw.AppendRows(failureTableRows)
	ftw.SetIndexColumn(1)

	stw := table.NewWriter()
	stw.AppendHeader(successNodeRowHeader)
	stw.AppendRows(successTableRows)
	stw.SetIndexColumn(1)

	return generateOutputString(cfg, ftw, stw)
}

func generateReport(results []*ScanResults, cfg *Config) (string, string) {
	ftw, stw := renderReport(results)
	failureReport, successReport := generateOutputString(cfg, ftw, stw)
	return failureReport, successReport
}

func generateOutputString(cfg *Config, ftw table.Writer, stw table.Writer) (string, string) {
	var failureReport string
	switch cfg.OutputFormat {
	case "table":
		failureReport = ftw.Render()
	case "csv":
		failureReport = ftw.RenderCSV()
	case "markdown":
		failureReport = ftw.RenderMarkdown()
	case "html":
		failureReport = ftw.RenderHTML()
	}

	var successReport string
	switch cfg.OutputFormat {
	case "table":
		successReport = stw.Render()
	case "csv":
		successReport = stw.RenderCSV()
	case "markdown":
		successReport = stw.RenderMarkdown()
	case "html":
		successReport = stw.RenderHTML()
	}
	return failureReport, successReport
}

func getComponent(res *ScanResult) *OpenshiftComponent {
	if res.Component != nil {
		return res.Component
	}
	return &OpenshiftComponent{
		Component: "<unknown>",
	}
}

func renderReport(results []*ScanResults) (failures table.Writer, successes table.Writer) {
	var failureTableRows []table.Row
	var successTableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			component := getComponent(res)
			if res.Error != nil {
				failureTableRows = append(failureTableRows, table.Row{component.Component, res.Tag.Name, res.Path, res.Error, res.Tag.From.Name})
			} else {
				successTableRows = append(successTableRows, table.Row{component.Component, res.Tag.Name, res.Path, res.Tag.From.Name})
			}
		}
	}

	ftw := table.NewWriter()
	ftw.AppendHeader(failureRowHeader)
	ftw.AppendRows(failureTableRows)
	ftw.SetIndexColumn(1)

	stw := table.NewWriter()
	stw.AppendHeader(successRowHeader)
	stw.AppendRows(successTableRows)
	stw.SetIndexColumn(1)
	return ftw, stw
}
