package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"k8s.io/klog/v2"

	mapset "github.com/deckarep/golang-set/v2"
)

const (
	colTitleOperatorName = "Operator Name"
	colTitleTagName      = "Tag Name"
	colTitleRpmName      = "RPM Name"
	colTitleExeName      = "Executable Name"
	colTitlePassedFailed = "Status"
	colTitleImage        = "Image"
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

	klog.InfoS("files stats", "total", total.Load(), "scanned", scanned.Load(), "passed", passed.Load())

	if cfg.PrintExceptions {
		displayExceptions(results)
	}
}

func getExceptionPrefix(res *ScanResult) string {
	if res.Rpm != "" {
		return "rpm." + res.Rpm
	}
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
			prefix := getExceptionPrefix(res)
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

	failureNodeRowHeader := table.Row{colTitleRpmName, colTitleExeName, colTitlePassedFailed}
	successNodeRowHeader := table.Row{colTitleRpmName, colTitleExeName}

	for _, result := range results {
		for _, res := range result.Items {
			if res.Error != nil {
				failureTableRows = append(failureTableRows, table.Row{res.Rpm, res.Path, res.Error})
			} else {
				successTableRows = append(successTableRows, table.Row{res.Rpm, res.Path})
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
	return ""
}

func getTag(res *ScanResult) string {
	if res.Tag != nil {
		return res.Tag.Name
	}
	return ""
}

func renderReport(results []*ScanResults) (failures table.Writer, successes table.Writer) {
	var failureTableRows []table.Row
	var successTableRows []table.Row

	failureRowHeader := table.Row{colTitleOperatorName, colTitleTagName, colTitleRpmName, colTitleExeName, colTitlePassedFailed, colTitleImage}
	successRowHeader := table.Row{colTitleOperatorName, colTitleTagName, colTitleRpmName, colTitleExeName, colTitleImage}

	for _, result := range results {
		for _, res := range result.Items {
			component := getComponent(res)
			tag := getTag(res)
			if res.Error != nil {
				failureTableRows = append(failureTableRows, table.Row{component, tag, res.Rpm, res.Path, res.Error, res.Tag.From.Name})
			} else {
				successTableRows = append(successTableRows, table.Row{component, tag, res.Rpm, res.Path, res.Tag.From.Name})
			}
		}
	}

	ftw := table.NewWriter()
	ftw.SuppressEmptyColumns()
	ftw.AppendHeader(failureRowHeader)
	ftw.AppendRows(failureTableRows)
	ftw.SetIndexColumn(1)

	stw := table.NewWriter()
	stw.SuppressEmptyColumns()
	stw.AppendHeader(successRowHeader)
	stw.AppendRows(successTableRows)
	stw.SetIndexColumn(1)
	return ftw, stw
}
