package main

import (
	"fmt"
	"os"
	"sort"

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

	colTitleNodePath         = "Path"
	colTitleNodePassedFailed = "Status"
	colTitleNodeFrom         = "From"
	failureNodeRowHeader     = table.Row{colTitleNodePath, colTitleNodePassedFailed, colTitleNodeFrom}
	successNodeRowHeader     = table.Row{colTitleNodePath}
)

func printResults(cfg *Config, results []*ScanResults, nodeScan bool) {
	var failureReport, successReport string

	var combinedReport string

	if nodeScan {
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

	if cfg.PrintExceptions {
		displayExceptions(results)
	}
}

func getPayloadOrTagPrefix(res *ScanResult) string {
	if res.Component != nil && res.Component.Component != "" {
		return "payload." + res.Component.Component
	}
	if res.Tag != nil && res.Tag.Name != "" {
		return "tag." + res.Tag.Name
	}
	return ""
}

func displayExceptions(results []*ScanResults) {
	exceptions := make(map[string]map[string]mapset.Set[*ScanResult])
	for _, result := range results {
		for _, res := range result.Items {
			if res.Error == nil {
				// skip over successes
				continue
			}
			prefix := getPayloadOrTagPrefix(res)
			errMap, ok := exceptions[prefix]
			if !ok {
				errMap = make(map[string]mapset.Set[*ScanResult])
				exceptions[prefix] = errMap
			}

			errName := getErrName(res.Error)
			if set, ok := errMap[errName]; ok {
				set.Add(res)
			} else {
				errMap[errName] = mapset.NewSet(res)
			}
		}
	}

	for prefix, errMap := range exceptions {
		for errName, set := range errMap {
			if prefix != "" {
				fmt.Printf("[[%v.ignore_errors]]\n", prefix)
			} else {
				fmt.Println("[[ignore_errors]]")
			}
			fmt.Printf("error = %q\n", errName)
			ss := set.ToSlice()
			sort.Slice(ss, func(i, j int) bool {
				return ss[i].Path < ss[j].Path
			})
			if len(ss) == 1 {
				fmt.Printf("files = [ %q ]\n", ss[0].Path)
			} else {
				fmt.Println("files = [")
				for _, res := range ss {
					fmt.Printf("  %q,\n", res.Path)
				}
				fmt.Println("]")
			}
			fmt.Println("")
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

func getComponent(res *ScanResult) string {
	if res.Component != nil {
		return res.Component.Component
	}
	return "<unknown>"
}

func renderReport(results []*ScanResults) (failures table.Writer, successes table.Writer) {
	var failureTableRows []table.Row
	var successTableRows []table.Row

	for _, result := range results {
		for _, res := range result.Items {
			component := getComponent(res)
			if res.Error != nil {
				failureTableRows = append(failureTableRows, table.Row{component, res.Tag.Name, res.Path, res.Error, res.Tag.From.Name})
			} else {
				successTableRows = append(successTableRows, table.Row{component, res.Tag.Name, res.Path, res.Tag.From.Name})
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
