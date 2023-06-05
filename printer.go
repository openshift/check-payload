package main

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	colTitleNamespace    = "Namespace"
	colTitlePodName      = "Pod Name"
	colTitleExeName      = "Executable Name"
	colTitlePassedFailed = "Status"
	rowHeader            = table.Row{colTitleNamespace, colTitlePodName, colTitleExeName, colTitlePassedFailed}
)

func printResults(cfg *Config, results []*ScanResults) error {
	var tableRows []table.Row

	fmt.Println("---")

	for _, result := range results {
		for _, res := range result.Items {
			if res.Error != nil {
				tableRows = append(tableRows, table.Row{res.PodNamespace, res.PodName, res.Path, res.Error})
			}
		}
	}

	tw := table.NewWriter()
	tw.AppendHeader(rowHeader)
	tw.AppendRows(tableRows)
	tw.SetIndexColumn(1)

	var report string
	switch cfg.OutputFormat {
	case "table":
		report = tw.Render()
	case "csv":
		report = tw.RenderCSV()
	case "markdown":
		report = tw.RenderMarkdown()
	case "html":
		report = tw.RenderHTML()
	}

	fmt.Println(report)

	if cfg.OutputFile != "" {
		if err := os.WriteFile(cfg.OutputFile, []byte(report), 0); err != nil {
			return fmt.Errorf("could not write file %v : %v", cfg.OutputFile, err)
		}
	}
	return nil
}
