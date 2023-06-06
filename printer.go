package main

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	colTitleTagName      = "Tag Name"
	colTitleExeName      = "Executable Name"
	colTitlePassedFailed = "Status"
	colTitleImage        = "Image"
	colTitleUsingCrypto  = "Using Crypto"
	rowHeader            = table.Row{colTitleTagName, colTitleExeName, colTitlePassedFailed, colTitleImage}
)

func printResults(cfg *Config, results []*ScanResults) error {
	failureReport, successReport := generateReport(results, cfg)

	var combinedReport string
	fmt.Println("---- Failure Report")
	fmt.Println(failureReport)
	combinedReport = failureReport

	if cfg.Verbose {
		fmt.Println("---- Success Report")
		fmt.Println(successReport)
		combinedReport += "\n\n ---- Success Report\n" + successReport
	}

	if cfg.OutputFile != "" {
		if err := os.WriteFile(cfg.OutputFile, []byte(combinedReport), 0770); err != nil {
			return fmt.Errorf("could not write file %v : %v", cfg.OutputFile, err)
		}
	}
	return nil
}

func generateReport(results []*ScanResults, cfg *Config) (string, string) {
	ftw, stw := renderFailures(results)

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

func renderFailures(results []*ScanResults) (failures table.Writer, successes table.Writer) {
	var failureTableRows []table.Row
	var successTableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			if res.Error != nil {
				failureTableRows = append(failureTableRows, table.Row{res.Tag.Name, res.Path, res.Error, res.Tag.From.Name})
			} else {
				successTableRows = append(successTableRows, table.Row{res.Tag.Name, res.Path, "", res.Tag.From.Name})
			}
		}
	}

	ftw := table.NewWriter()
	ftw.AppendHeader(rowHeader)
	ftw.AppendRows(failureTableRows)
	ftw.SetIndexColumn(1)

	stw := table.NewWriter()
	stw.AppendHeader(rowHeader)
	stw.AppendRows(successTableRows)
	stw.SetIndexColumn(1)
	return ftw, stw
}
