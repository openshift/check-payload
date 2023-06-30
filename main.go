package main

import (
	_ "embed"
	"flag"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/openshift/check-payload/internal/cli"
	"github.com/openshift/check-payload/internal/scan"
)

//go:embed config.toml
var embeddedConfig string

var (
	Commit  string
	verbose bool
)

func main() {
	var config scan.Config
	var results []*scan.ScanResults

	rootCmd := cobra.Command{
		Use:           "check-payload",
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose")

	versionCmd := cli.NewVersionCmd(Commit)
	scanCmd := cli.NewScanCmd(embeddedConfig, Commit, verbose, &config, &results)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(scanCmd)

	// Add klog flags.
	klogFlags := flag.NewFlagSet("", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	rootCmd.PersistentFlags().AddGoFlagSet(klogFlags)

	if err := rootCmd.Execute(); err != nil {
		klog.Fatalf("Error: %v\n", err)
	}
}
