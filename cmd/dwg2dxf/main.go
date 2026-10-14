// Command dwg2dxf converts a DWG drawing to DXF.
//
// Usage:
//
//	dwg2dxf [-o out.dxf] [-version R2018] [-summary] <file.dwg>
//
// With no -o the DXF goes to stdout.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/flywave/go-dwg/dxf"
)

func main() {
	out := flag.String("o", "", "output DXF path (default stdout)")
	verFlag := flag.String("version", "R2018", "target DXF dialect (R12…R2018 or AC1009…AC1032)")
	summary := flag.Bool("summary", false, "print a conversion summary to stderr")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: dwg2dxf [-o out.dxf] [-version R2018] [-summary] <file.dwg>")
		os.Exit(2)
	}

	version, ok := dxf.ParseDxfVersion(*verFlag)
	if !ok {
		fmt.Fprintf(os.Stderr, "dwg2dxf: unknown version %q\n", *verFlag)
		os.Exit(2)
	}

	doc, err := dxf.ConvertDwgFile(flag.Arg(0), version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dwg2dxf: %v\n", err)
		os.Exit(1)
	}

	if *summary {
		kinds := map[string]int{}
		for _, e := range doc.Entities {
			kinds[e.Type]++
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "source:    %s\n", flag.Arg(0))
		fmt.Fprintf(&sb, "target:    DXF %s (%s)\n", version, version.AcadVer())
		fmt.Fprintf(&sb, "entities:  %d\n", len(doc.Entities))
		fmt.Fprintf(&sb, "blocks:    %d\n", len(doc.Blocks))
		fmt.Fprintf(&sb, "layers:    %d\n", len(doc.LayerNames()))
		fmt.Fprintf(&sb, "objects:   %d\n", len(doc.Objects))
		for k, n := range kinds {
			fmt.Fprintf(&sb, "  %-12s %d\n", k, n)
		}
		fmt.Fprint(os.Stderr, sb.String())
	}

	if *out == "" {
		if err := doc.SaveTo(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "dwg2dxf: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := doc.Save(*out); err != nil {
		fmt.Fprintf(os.Stderr, "dwg2dxf: %v\n", err)
		os.Exit(1)
	}
}
