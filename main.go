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

	"github.com/openshift/check-payload/dist/releases"
	"github.com/openshift/check-payload/internal/scan"
	"github.com/openshift/check-payload/internal/types"
)

const (
	defaultPayloadFilename = "payload.json"
	defaultConfigFile      = "config.toml"
)

//go:embed config.toml
var embeddedConfig string

var applicationDeps = []string{
	"nm",
	"oc",
	"podman",
}

var applicationDepsNodeScan = []string{
	"nm",
	"rpm",
}

var Commit string

var (
	components                            []string
	configFile, configForVersion          string
	cpuProfile                            string
	failOnWarnings                        bool
	filterFiles, filterDirs, filterImages []string
	javaDisabledAlgorithms                []string
	insecurePull                          bool
	limit                                 int
	outputFile                            string
	outputFormat                          string
	parallelism                           int
	printExceptions                       bool
	pullSecretFile                        string
	timeLimit                             time.Duration
	verbose                               bool
	localBundlePath                       string
)

func main() {
	var config types.Config
	var results []*types.ScanResults

	rootCmd := cobra.Command{
		Use:           "check-payload",
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Println(Commit)
			return nil
		},
	}

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Run a scan",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := getConfig(&config.ConfigFile); err != nil {
				return err
			}
			config.FailOnWarnings = failOnWarnings
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
			config.Components = components
			klog.InfoS("scan", "version", Commit)

			// Validate the configuration.
			err, warn := config.Validate()
			if warn != nil {
				klog.Warning(warn)
			}
			if err != nil {
				return fmt.Errorf("config has bad entries, please fix: %w", err)
			}

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
		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			if cpuProfile != "" {
				pprof.StopCPUProfile()
				klog.Info("CPU profile saved to ", cpuProfile)
			}
			scan.PrintResults(&config, results)
			if scan.IsFailed(results) {
				return errors.New("run failed")
			}
			if scan.IsWarnings(results) && config.FailOnWarnings {
				return errors.New("run failed with warnings")
			}
			return nil
		},
	}
	scanCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "use toml config file (default: "+defaultConfigFile+")")
	scanCmd.PersistentFlags().StringVarP(&configForVersion, "config-for-version", "V", "", "use embedded toml config file for specified version")
	scanCmd.PersistentFlags().StringSliceVar(&filterFiles, "filter-files", nil, "")
	scanCmd.PersistentFlags().StringSliceVar(&filterDirs, "filter-dirs", nil, "")
	scanCmd.PersistentFlags().StringSliceVar(&filterImages, "filter-images", nil, "")
	scanCmd.PersistentFlags().StringSliceVar(&components, "components", nil, "Filter scans by component. Payload scans support a list of components. Local scans support at most one component, which is intended to match the local unpacked image.")
	scanCmd.PersistentFlags().BoolVar(&failOnWarnings, "fail-on-warnings", false, "fail on warnings")
	scanCmd.PersistentFlags().BoolVar(&insecurePull, "insecure-pull", false, "use insecure pull")
	scanCmd.PersistentFlags().IntVar(&limit, "limit", -1, "limit the number of pods scanned")
	scanCmd.PersistentFlags().IntVar(&parallelism, "parallelism", 5, "how many pods to check at once")
	scanCmd.PersistentFlags().StringVar(&outputFile, "output-file", "", "write report to file")
	scanCmd.PersistentFlags().StringVar(&outputFormat, "output-format", "table", "output format (table, csv, markdown, html)")
	scanCmd.PersistentFlags().StringVar(&pullSecretFile, "pull-secret", "", "pull secret to use for pulling images")
	scanCmd.PersistentFlags().DurationVar(&timeLimit, "time-limit", 1*time.Hour, "limit running time")
	scanCmd.PersistentFlags().StringVar(&cpuProfile, "cpuprofile", "", "write CPU profile to file")
	scanCmd.PersistentFlags().BoolVarP(&printExceptions, "print-exceptions", "p", false, "display exception list")

	scanPayload := &cobra.Command{
		Use:          "payload [image pull spec]",
		SilenceUsage: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return scan.ValidateApplicationDependencies(applicationDeps)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeLimit)
			defer cancel()
			config.FromURL, _ = cmd.Flags().GetString("url")
			config.FromFile, _ = cmd.Flags().GetString("file")
			if config.FromURL == "" && config.FromFile == "" {
				return errors.New("either -u, --url or -f, --file option is required")
			}
			config.PrintExceptions, _ = cmd.Flags().GetBool("print-exceptions")
			config.UseRPMScan, _ = cmd.Flags().GetBool("rpm-scan")
			results = scan.RunPayloadScan(ctx, &config)
			return nil
		},
	}
	scanPayload.Flags().StringP("url", "u", "", "payload url")
	scanPayload.Flags().StringP("file", "f", "", "payload from json file")
	scanPayload.MarkFlagsMutuallyExclusive("url", "file")
	scanPayload.Flags().Bool("rpm-scan", false, "use RPM scan (same as during node scan)")

	// Define the 'local' subcommand for scanning local unpacked images
	localCmd := &cobra.Command{
		Use:   "local",
		Short: "Scan a local unpacked image bundle",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if localBundlePath == "" {
				return fmt.Errorf("path to local bundle is required")
			}
			// If the user passes in more than one component for a
			// local scan, we're dealing with too much ambiguity.
			// Otherwise, the local scan needs to run for each
			// component supplied, when it only maps to a single
			// component. In this case, the user needs to either
			// use an empty list (no component filtering) or reduce
			// their filter to a list of one to remove ambiguity
			// and allow check payload to safely determine the
			// component to filter against.
			if len(components) > 1 {
				return fmt.Errorf("local scans do not support multiple components, use one or none: %s", components)
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), config.TimeLimit)
			defer cancel()
			results = scan.RunLocalScan(ctx, &config, localBundlePath)
			return nil
		},
	}

	localCmd.Flags().StringVar(&localBundlePath, "path", "", "Path to the local unpacked image bundle")

	// Add the 'local' subcommand to 'scanCmd'
	scanCmd.AddCommand(localCmd)

	scanNode := &cobra.Command{
		Use:          "node --root /myroot [--walk-scan]",
		SilenceUsage: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return scan.ValidateApplicationDependencies(applicationDepsNodeScan)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeLimit)
			defer cancel()
			root, _ := cmd.Flags().GetString("root")
			walkScan, _ := cmd.Flags().GetBool("walk-scan")
			config.UseRPMScan = !walkScan
			results = scan.RunNodeScan(ctx, &config, root)
			return nil
		},
	}
	scanNode.Flags().String("root", "", "root path to scan")
	scanNode.Flags().Bool("walk-scan", false, "scan all files using directory tree walk")
	_ = scanNode.MarkFlagRequired("root")

	scanImage := &cobra.Command{
		Use:          "image [image pull spec]",
		Aliases:      []string{"operator"},
		SilenceUsage: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return scan.ValidateApplicationDependencies(applicationDeps)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeLimit)
			defer cancel()
			config.ContainerImage, _ = cmd.Flags().GetString("spec")
			config.UseRPMScan, _ = cmd.Flags().GetBool("rpm-scan")
			results = scan.RunOperatorScan(ctx, &config)
			return nil
		},
	}
	scanImage.Flags().String("spec", "", "payload url")
	scanImage.Flags().Bool("rpm-scan", false, "use RPM scan (same as during node scan)")
	_ = scanImage.MarkFlagRequired("spec")

	scanJavaImage := &cobra.Command{
		Use:          "java-image [image pull spec]",
		Aliases:      []string{"java"},
		SilenceUsage: true,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return scan.ValidateApplicationDependencies(applicationDeps)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), timeLimit)
			defer cancel()
			config.ContainerImage, _ = cmd.Flags().GetString("spec")
			config.UseRPMScan, _ = cmd.Flags().GetBool("rpm-scan")
			config.JavaDisabledAlgorithms = append(config.JavaDisabledAlgorithms, javaDisabledAlgorithms...)
			config.Java = true
			results = scan.RunOperatorScan(ctx, &config)
			return nil
		},
	}
	scanJavaImage.Flags().String("spec", "", "java payload url")
	scanJavaImage.Flags().Bool("rpm-scan", false, "use RPM scan (same as during node scan)")
	scanJavaImage.Flags().StringSliceVar(&javaDisabledAlgorithms, "disabled-algorithms", nil, "additional algorithms that java should be disabling in FIPS mode")
	_ = scanJavaImage.MarkFlagRequired("spec")

	scanCmd.AddCommand(scanPayload)
	scanCmd.AddCommand(scanJavaImage)
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

func getConfig(config *types.ConfigFile) error {
	// Handle --config.
	file := configFile
	if file == "" {
		file = defaultConfigFile
	}
	res, err := toml.DecodeFile(file, &config)
	if err == nil {
		klog.Infof("using config file: %v", file)
		if un := res.Undecoded(); len(un) != 0 {
			return fmt.Errorf("unknown keys in config: %+v", un)
		}
	} else if errors.Is(err, os.ErrNotExist) && configFile == "" {
		// When --config not specified and defaultConfigFile is not found,
		// fall back to embedded config.
		klog.Info("using embedded config")
		res, err = toml.Decode(embeddedConfig, &config)
		if err != nil { // Should never happen.
			panic("invalid embedded config: " + err.Error())
		}
		if un := res.Undecoded(); len(un) != 0 {
			panic(fmt.Errorf("unknown keys in config: %+v", un))
		}
	} else {
		// Otherwise, error out.
		return fmt.Errorf("can't parse config file %q: %w", file, err)
	}

	if configForVersion != "" {
		// Append to the main config.
		cfg, err := releases.GetConfigFor(configForVersion)
		if err != nil {
			return err
		}
		klog.Infof("adding rules from embedded config for %s", configForVersion)
		addConfig := &types.ConfigFile{}
		res, err = toml.Decode(string(cfg), &addConfig)
		if err != nil { // Should never happen.
			panic("invalid embedded config: " + err.Error())
		}
		if un := res.Undecoded(); len(un) != 0 {
			panic(fmt.Errorf("unknown keys in config: %+v", un))
		}
		if warn := config.Add(addConfig); warn != nil {
			klog.Warning(warn)
		}
	}

	return nil
}
