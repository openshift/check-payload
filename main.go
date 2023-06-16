package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
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

func main() {
	var containerImage = flag.String("container-image", "", "only run scan on operator image")
	var components = flag.String("components", "", "scan a specific set of components")
	var configFile = flag.String("config", defaultConfigFile, "use config file")
	var filterImages = flag.String("filter-images", "", "filter images")
	var fromFile = flag.String("file", defaultPayloadFilename, "json file for payload")
	var fromUrl = flag.String("url", "", "http URL to pull payload from")
	var help = flag.Bool("help", false, "show help")
	var limit = flag.Int("limit", 0, "limit the number of pods scanned")
	var outputFile = flag.String("output-file", "", "write report to this file")
	var outputFormat = flag.String("output-format", "table", "output format (table, csv, markdown, html)")
	var parallelism = flag.Int("parallelism", 5, "how many pods to check at once")
	var timeLimit = flag.Duration("time-limit", 1*time.Hour, "limit running time")
	var verbose = flag.Bool("verbose", false, "verbose")
	var filter = flag.String("filter", "", "do not scan a specific directory")
	var nodeScan = flag.String("node-scan", "", "scan a node, pass / to scan the root or pass a path for a different start point")
	var version = flag.Bool("version", false, "print version")
	var insecurePull = flag.Bool("insecure-pull", false, "allow for insecure podman pulls")

	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}
	if *version {
		fmt.Println(Commit)
		os.Exit(0)
	}

	config := Config{
		ConfigFile:     *configFile,
		FromFile:       *fromFile,
		FromURL:        *fromUrl,
		InsecurePull:   *insecurePull,
		Limit:          *limit,
		NodeScan:       *nodeScan,
		ContainerImage: *containerImage,
		OutputFile:     *outputFile,
		OutputFormat:   *outputFormat,
		Parallelism:    *parallelism,
		TimeLimit:      *timeLimit,
		Verbose:        *verbose,
	}

	if *components != "" {
		config.Components = strings.Split(*components, ",")
	}

	if *filterImages != "" {
		config.FilterImages = strings.Split(*filterImages, ",")
	}

	if *configFile != "" {
		var tomlData string
		if filterFileData, err := os.ReadFile(*configFile); err != nil {
			klog.Info("using embedded config")
			tomlData = embeddedConfig
		} else {
			klog.Infof("using provided config: %v", *configFile)
			tomlData = string(filterFileData)
		}
		_, err := toml.Decode(tomlData, &config)
		if err != nil {
			klog.Fatalf("error parsing toml: %v", err)
		}
	}

	if *filter != "" {
		config.FilterPaths = append(config.FilterPaths, strings.Split(*filter, ",")...)
	}

	klog.InitFlags(nil)

	if err := validateApplicationDependencies(&config); err != nil {
		klog.Fatalf("%+v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeLimit)
	defer cancel()

	config.Log()

	results := Run(ctx, &config)
	err := printResults(&config, results)
	if err != nil || isFailed(results) {
		os.Exit(1)
	}
}
