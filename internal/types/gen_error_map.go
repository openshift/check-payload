//go:build ignore

package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"
)

var (
	inFile  = flag.String("in", "errors.go", "Input file name")
	outFile = flag.String("out", "", "Output file name")

	tmpl = template.Must(template.New("").Parse(`// Code generated from {{ .Input }} using 'go generate'; DO NOT EDIT.

package types

var KnownErrors = map[string]error {
{{- range .Vars }}
	{{ printf "\"%s\": %s," . . }}
{{- end }}
}
`))
)

func main() {
	flag.Parse()

	input, err := os.Open(*inFile)
	if err != nil {
		log.Fatal(err)
	}
	defer input.Close()

	out := os.Stdout
	if *outFile != "" {
		out, err = os.Create(*outFile)
		if err != nil {
			log.Fatal(err)
		}
		defer out.Close()
	}

	vars := []string{}
	s := bufio.NewScanner(input)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "\tErr") && strings.Contains(line, " = errors.New(") {
			// Extract the variable name.
			vars = append(vars, line[1:strings.IndexByte(line, ' ')])
		}
	}
	if err := s.Err(); err != nil {
		log.Fatal(err)
	}
	sort.Strings(vars)
	tmpl.Execute(out, struct {
		Input string
		Vars  []string
	}{
		Input: *inFile,
		Vars:  vars,
	})
}
