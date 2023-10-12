package main

import (
	"bufio"
	"bytes"
	"debug/buildinfo"
	"debug/elf"
	"debug/gosym"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// From go/src/debug/gosym/pclntab.go
const (
	go12magic  = 0xfffffffb
	go116magic = 0xfffffffa
	go118magic = 0xfffffff0
	go120magic = 0xfffffff1
)

// Print all function names defined in .gopclntab
func printFuncs(symTable *gosym.Table) {
	for _, f := range symTable.Funcs {
		fmt.Printf("%s\n", f.Name)
	}
}

// Select the magic number based on the Go version
func magicNumber(goVersion string) []byte {
	bs := make([]byte, 4)
	var magic uint32
	if strings.Compare(goVersion, "go1.20") >= 0 {
		magic = go120magic
	} else if strings.Compare(goVersion, "go1.18") >= 0 {
		magic = go118magic
	} else if strings.Compare(goVersion, "go1.16") >= 0 {
		magic = go116magic
	} else {
		magic = go12magic
	}
	binary.LittleEndian.PutUint32(bs, magic)
	return bs
}

// Open a Go ELF executable and read .gopclntab
func readTable(fileName string) *gosym.Table {
	// Default section label is .gopclntab
	sectionLabel := ".gopclntab"
	bi, err := buildinfo.ReadFile(fileName)
	if err != nil {
		log.Fatalf("could not read buildinfo from %s", fileName)
	}

	exe, err := elf.Open(fileName)
	if err != nil {
		log.Fatalf("could not read %s as ELF", fileName)
	}

	defer exe.Close()
	section := exe.Section(sectionLabel)
	if section == nil {
		// binary may be built with -pie
		sectionLabel = ".data.rel.ro"
		section = exe.Section(sectionLabel)
		if section == nil {
			log.Fatalf("could not read section .gopclntab from %s ", fileName)
		}
	}
	tableData, err := section.Data()
	if err != nil {
		log.Fatalf("found section but could not read .gopclntab from %s ", fileName)
	}

	// Find .gopclntab by magic number even if there is no section label
	magic := magicNumber(bi.GoVersion)
	pclntabIndex := bytes.Index(tableData, magic)
	if pclntabIndex < 0 {
		log.Fatalf("could not find magic number in %s ", fileName)
	}
	tableData = tableData[pclntabIndex:]
	addr := exe.Section(".text").Addr
	lineTable := gosym.NewLineTable(tableData, addr)
	symTable, err := gosym.NewTable([]byte{}, lineTable)
	if err != nil {
		log.Fatalf("could not create symbol table from  %s ", fileName)
	}
	return symTable
}

// Exit due to invalid arguments
func exitCmdLineError(err string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	printCmdUsage()
	os.Exit(1)
}

// Print the help menu
func printCmdUsage() {
	writer := flag.CommandLine.Output()
	splitPath := strings.Split(os.Args[0], "/")
	cmd := splitPath[len(splitPath)-1]
	fmt.Fprintf(writer, "Usage: %s <option> <go binary>\n", cmd)
	flag.PrintDefaults()
}

// Returns true if exactly one argument is true,
// Otherwise returns false
func exactlyOneOf(bools ...bool) bool {
	if len(bools) < 1 {
		panic("expected nonzero list")
	}
	acc := false
	for _, b := range bools {
		// multiple true elements case
		if acc && b {
			return false
		}
		acc = acc || b
	}
	return acc
}

// Reads a newline delimited file of expected symbol names.
// Returns a list of symbol names with whitespace trimmed.
func readExpectedSymsFile(file io.Reader) []string {
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}
	return lines
}

// Checks the .gopclntab for the expected list of symbols.
// Fails if any expected symbol is not found.
func assertExpectedSyms(expectedSyms []string, symTable *gosym.Table) {
	for _, s := range expectedSyms {
		fn := symTable.LookupFunc(s)
		if fn == nil {
			log.Fatalf("symbol not found: %s", s)
		}
		fmt.Fprintf(os.Stderr, "found symbol %s\n", s)
	}
}

func main() {
	// Define flags
	listFlag := flag.Bool("list", false, "print a list of function symbols in .gopclntab")
	expectFlag := flag.String("expect", "", "assert a comma separated list of symobls")
	expectStdinFlag := flag.Bool("expect-stdin", false, "assert a newline delimited list of symbols from stdin")
	expectFileFlag := flag.String("expect-file", "", "assert a newline delimited list of symbols from file")

	// Parse flags
	flag.Usage = printCmdUsage
	flag.Parse()
	if len(flag.Args()) != 1 {
		exitCmdLineError("expected exactly one argument (executable file)")
	}

	// Check validity of flags
	mutuallyExclusiveFlags := []bool{
		*listFlag,
		(*expectFlag != ""),
		*expectStdinFlag,
		(*expectFileFlag != ""),
	}
	if !exactlyOneOf(mutuallyExclusiveFlags...) {
		exitCmdLineError("multiple options specified")
	}

	// Read .gopclntab from binary
	fileName := flag.Arg(0)
	symTable := readTable(fileName)

	// Dispatch to specified routine
	if *listFlag {
		printFuncs(symTable)
	} else if *expectFlag != "" {
		expectedSyms := strings.Split(*expectFlag, ",")
		assertExpectedSyms(expectedSyms, symTable)
	} else if *expectStdinFlag {
		expectedSyms := readExpectedSymsFile(os.Stdin)
		assertExpectedSyms(expectedSyms, symTable)
	} else if *expectFileFlag != "" {
		file, err := os.Open(*expectFileFlag)
		if err != nil {
			log.Fatalf("error: %s", err)
		}
		defer file.Close()
		expectedSyms := readExpectedSymsFile(file)
		assertExpectedSyms(expectedSyms, symTable)
	} else {
		exitCmdLineError("no options specified")
	}
}
