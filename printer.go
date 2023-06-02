package main

import (
	"fmt"

	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	colTitleIndex        = "#"
	colTitleExeName      = "Executable Name"
	colTitlePassedFailed = "Passed/Failed"
	rowHeader            = table.Row{colTitleExeName, colTitlePassedFailed}
)

func printResults(results []*ScanResults) {
	var tableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			tableRows = append(tableRows, table.Row{res.Path, res.ScanPassed})
		}
	}
	tw := table.NewWriter()
	tw.AppendHeader(rowHeader)
	tw.AppendRows(tableRows)
	tw.SetIndexColumn(1)

	fmt.Println(tw.Render())
}
