package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

const (
	defaultPayloadFilename = "payload.json"
	defaultConfigFile      = "config.toml"
)

//go:embed config.toml
var embeddedConfig string

var applicationDeps = []string{
	"file",
	"go",
	"nm",
	"oc",
	"podman",
	"readelf",
	"strings",
}

var applicationDepsNodeScan = []string{
	"file",
	"go",
	"nm",
	"readelf",
	"rpm",
	"strings",
}

var ignoredMimes = []string{
	"application/gzip",
	"application/json",
	"application/octet-stream",
	"application/tzif",
	"application/vnd.sqlite3",
	"application/x-sharedlib",
	"application/zip",
	"text/csv",
	"text/html",
	"text/plain",
	"text/tab-separated-values",
	"text/xml",
	"text/x-python",
}

var requiredGolangSymbols = []string{
	"vendor/github.com/golang-fips/openssl-fips/openssl._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
	"crypto/internal/boring._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
}

var Commit string

var (
	configFile                            string
	parallelism                           int
	outputFile                            string
	outputFormat                          string
	filterFiles, filterDirs, filterImages []string
	components                            []string
	insecurePull                          bool
	verbose                               bool
	limit                                 int
	timeLimit                             time.Duration
	cpuProfile                            string
)

func main() {
	var config Config
	var results []*ScanResults

	rootCmd := cobra.Command{
		Use:           "check-payload",
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(Commit)
			return nil
		},
	}

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Run a scan",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := getConfig(&config); err != nil {
				return err
			}
			config.FilterFiles = append(config.FilterFiles, filterFiles...)
			config.FilterDirs = append(config.FilterDirs, filterDirs...)
			config.FilterImages = append(config.FilterImages, filterImages...)
			config.Parallelism = parallelism
			config.InsecurePull = insecurePull
			config.OutputFile = outputFile
			config.OutputFormat = outputFormat
			config.Limit = limit
			config.TimeLimit = timeLimit
			config.Verbose = verbose
			config.Log()
			klog.InfoS("scan", "version", Commit)

			if cpuProfile != "" {
				f, err := os.Create(cpuProfile)
				if err != nil {
					return err
				}
				klog.Info("collecting CPU profile data to ", cpuProfile)
				pprof.StartCPUProfile(f)
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if cpuProfile != "" {
				pprof.StopCPUProfile()
				klog.Info("CPU profile saved to ", cpuProfile)
			}
			printResults(&config, results)
			if isFailed(results) {
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
	scanCmd.PersistentFlags().IntVar(&limit, "limit", -1, "limit the number of pods scanned")
	scanCmd.PersistentFlags().IntVar(&parallelism, "parallelism", 5, "how many pods to check at once")
	scanCmd.PersistentFlags().StringVar(&outputFile, "output-file", "", "write report to file")
	scanCmd.PersistentFlags().StringVar(&outputFormat, "output-format", "table", "output format (table, csv, markdown, html)")
	scanCmd.PersistentFlags().DurationVar(&timeLimit, "time-limit", 1*time.Hour, "limit running time")
	scanCmd.PersistentFlags().StringVar(&cpuProfile, "cpuprofile", "", "write CPU profile to file")

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
			results = runPayloadScan(ctx, &config)
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
			results = runNodeScan(ctx, &config)
			return nil
		},
	}
	scanNode.Flags().String("root", "", "root path to scan")
	scanNode.MarkFlagRequired("root")

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
			results = runOperatorScan(ctx, &config)
			return nil
		},
	}
	scanImage.Flags().String("spec", "", "payload url")
	scanImage.MarkFlagRequired("spec")

	scanCmd.AddCommand(scanPayload)
	scanCmd.AddCommand(scanNode)
	scanCmd.AddCommand(scanImage)

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

func getConfig(config *Config) error {
	file := configFile
	if file == "" {
		file = defaultConfigFile
	}
	_, err := toml.DecodeFile(file, &config)
	if err == nil {
		klog.Infof("using config file: %v", file)
		return nil
	}
	// When --config is not specified and defaultConfigFile is not found,
	// fall back to embedded config.
	if errors.Is(err, os.ErrNotExist) && configFile == "" {
		klog.Info("using embedded config")
		_, err = toml.Decode(string(embeddedConfig), &config)
		if err != nil { // Should never happen.
			panic("invalid embedded config: " + err.Error())
		}
		return nil
	}
	// Otherwise, error out.
	return fmt.Errorf("can't parse config file %q: %w", file, err)
}
