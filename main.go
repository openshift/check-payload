package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

const (
	defaultPayloadFilename = "payload.json"
)

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

var requiredGolangSymbolsGreaterThan1_18 = []string{
	"vendor/github.com/golang-fips/openssl-fips/openssl._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
	"crypto/internal/boring._Cfunc__goboringcrypto_DLOPEN_OPENSSL",
}

var requiredGolangSymbolsLessThan1_18 = []string{
	"x_cgo_init",
}

var Commit string

func main() {
	var operatorImage = flag.String("operator-image", "", "only run scan on operator image")
	var components = flag.String("components", "", "scan a specific set of components")
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
		FromFile:      *fromFile,
		FromURL:       *fromUrl,
		Limit:         *limit,
		NodeScan:      *nodeScan,
		OperatorImage: *operatorImage,
		OutputFile:    *outputFile,
		OutputFormat:  *outputFormat,
		Parallelism:   *parallelism,
		TimeLimit:     *timeLimit,
		Verbose:       *verbose,
	}

	if *components != "" {
		config.Components = strings.Split(*components, ",")
	}
	if *filter != "" {
		config.Filter = strings.Split(*filter, ",")
	}

	klog.InitFlags(nil)

	validateApplicationDependencies(&config)

	ctx, cancel := context.WithTimeout(context.Background(), *timeLimit)
	defer cancel()

	config.Log()

	results := Run(ctx, &config)
	err := printResults(&config, results)
	if err != nil || isFailed(results) {
		os.Exit(1)
	}
}
