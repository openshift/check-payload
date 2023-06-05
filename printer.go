package main

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	colTitleNamespace    = "Namespace"
	colTitlePodName      = "Pod Name"
	colTitleExeName      = "Executable Name"
	colTitlePassedFailed = "Status"
	rowHeader            = table.Row{colTitleNamespace, colTitlePodName, colTitleExeName, colTitlePassedFailed}
)

func printResults(cfg *Config, results []*ScanResults) {
	var tableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			if res.Error == nil {
				tableRows = append(tableRows, table.Row{res.PodNamespace, res.PodName, res.Path, "ok"})
			} else {
				tableRows = append(tableRows, table.Row{res.PodNamespace, res.PodName, res.Path, res.Error})
			}
		}
	}

	tw := table.NewWriter()
	tw.AppendHeader(rowHeader)
	tw.AppendRows(tableRows)
	tw.SetIndexColumn(1)

	switch cfg.OutputFormat {
	case "table":
		fmt.Println(tw.Render())
	case "csv":
		fmt.Println(tw.RenderCSV())
	case "markdown":
		fmt.Println(tw.RenderMarkdown())
	}
}
