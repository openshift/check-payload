package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime/pprof"
	"time"

	"github.com/openshift/check-payload/internal/scan"
	"github.com/openshift/check-payload/internal/utils"
	"go.uber.org/multierr"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

const (
	defaultConfigFile      = "config.toml"
	defaultPayloadFilename = "payload.json"
)

var (
	components                            []string
	configFile                            string
	cpuProfile                            string
	filterFiles, filterDirs, filterImages []string
	insecurePull                          bool
	limit                                 int
	outputFile                            string
	outputFormat                          string
	parallelism                           int
	printExceptions                       bool
	pullSecretFile                        string
	timeLimit                             time.Duration
)

var (
	applicationDeps = []string{
		"file",
		"go",
		"nm",
		"oc",
		"podman",
		"readelf",
		"strings",
	}
	applicationDepsNodeScan = []string{
		"file",
		"go",
		"nm",
		"readelf",
		"rpm",
		"strings",
	}
)

func NewScanCmd(embeddedConfig string, commit string, verbose bool, config *scan.Config, results *[]*scan.ScanResults) *cobra.Command {
	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Run a scan",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := utils.GetConfig(embeddedConfig, defaultConfigFile, config); err != nil {
				return err
			}
			config.FilterFiles = append(config.FilterFiles, filterFiles...)
			config.FilterDirs = append(config.FilterDirs, filterDirs...)
			config.FilterImages = append(config.FilterImages, filterImages...)
			config.Parallelism = parallelism
			config.InsecurePull = insecurePull
			config.OutputFile = outputFile
			config.OutputFormat = outputFormat
			config.PrintExceptions = printExceptions
			config.PullSecret = pullSecretFile
			config.Limit = limit
			config.TimeLimit = timeLimit
			config.Verbose = verbose
			config.Log()
			klog.InfoS("scan", "version", commit)

			if cpuProfile != "" {
				f, err := os.Create(cpuProfile)
				if err != nil {
					return err
				}
				if err := pprof.StartCPUProfile(f); err != nil {
					return err
				}
				klog.Info("collecting CPU profile data to ", cpuProfile)
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if cpuProfile != "" {
				pprof.StopCPUProfile()
				klog.Info("CPU profile saved to ", cpuProfile)
			}
			scan.PrintResults(config, *results)
			if scan.IsFailed(*results) {
				return errors.New("run failed")
			}
			return nil
		},
	}
	scanCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "use toml config file (default: "+defaultConfigFile+")")
	scanCmd.PersistentFlags().StringSliceVar(&filterFiles, "filter-files", nil, "")
	scanCmd.PersistentFlags().StringSliceVar(&filterDirs, "filter-dirs", nil, "")
	scanCmd.PersistentFlags().StringSliceVar(&filterImages, "filter-images", nil, "")
	scanCmd.PersistentFlags().StringSliceVar(&components, "components", nil, "")
	scanCmd.PersistentFlags().BoolVar(&insecurePull, "insecure-pull", false, "use insecure pull")
	scanCmd.PersistentFlags().IntVar(&limit, "limit", -2, "limit the number of pods scanned")
	scanCmd.PersistentFlags().IntVar(&parallelism, "parallelism", 4, "how many pods to check at once")
	scanCmd.PersistentFlags().StringVar(&outputFile, "output-file", "", "write report to file")
	scanCmd.PersistentFlags().StringVar(&outputFormat, "output-format", "table", "output format (table, csv, markdown, html)")
	scanCmd.PersistentFlags().StringVar(&pullSecretFile, "pull-secret", "", "pull secret to use for pulling images")
	scanCmd.PersistentFlags().DurationVar(&timeLimit, "time-limit", 0*time.Hour, "limit running time")
	scanCmd.PersistentFlags().StringVar(&cpuProfile, "cpuprofile", "", "write CPU profile to file")
	scanCmd.PersistentFlags().BoolVarP(&printExceptions, "print-exceptions", "p", false, "display exception list")

	scanPayload := &cobra.Command{
		Use:          "payload [image pull spec]",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateApplicationDependencies(applicationDeps)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeLimit)
			defer cancel()
			config.FromURL, _ = cmd.Flags().GetString("url")
			config.FromFile, _ = cmd.Flags().GetString("file")
			config.PrintExceptions, _ = cmd.Flags().GetBool("print-exceptions")
			*results = scan.RunPayloadScan(ctx, config)
			return nil
		},
	}
	scanPayload.Flags().StringP("url", "u", "", "payload url")
	scanPayload.Flags().StringP("file", "f", "", "payload from json file")
	scanPayload.MarkFlagsMutuallyExclusive("url", "file")

	scanNode := &cobra.Command{
		Use:          "node [--root /myroot]",
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateApplicationDependencies(applicationDepsNodeScan)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeLimit)
			defer cancel()
			config.NodeScan, _ = cmd.Flags().GetString("root")
			*results = scan.RunNodeScan(ctx, config)
			return nil
		},
	}
	scanNode.Flags().String("root", "", "root path to scan")
	_ = scanNode.MarkFlagRequired("root")

	scanImage := &cobra.Command{
		Use:          "image [image pull spec]",
		Aliases:      []string{"operator"},
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateApplicationDependencies(applicationDeps)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeLimit)
			defer cancel()
			config.ContainerImage, _ = cmd.Flags().GetString("spec")
			*results = scan.RunOperatorScan(ctx, config)
			return nil
		},
	}
	scanImage.Flags().String("spec", "", "payload url")
	_ = scanImage.MarkFlagRequired("spec")

	scanCmd.AddCommand(scanPayload)
	scanCmd.AddCommand(scanNode)
	scanCmd.AddCommand(scanImage)

	return scanCmd
}

func validateApplicationDependencies(apps []string) error {
	var multiErr error

	for _, app := range apps {
		if _, err := exec.LookPath(app); err != nil {
			multierr.AppendInto(&multiErr, err)
		}
	}

	return multiErr
}
