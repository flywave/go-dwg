// Command dxfinfo prints a JSON summary of a DXF file: dialect, header
// highlights, table rosters, entity histogram and object counts.
//
// Usage:
//
//	dxfinfo [-entities] <file.dxf>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/flywave/go-dwg/dxf"
)

type tableJSON struct {
	Name    string   `json:"name"`
	Entries []string `json:"entries"`
}

type summaryJSON struct {
	File       string         `json:"file"`
	Format     string         `json:"format"`
	Version    string         `json:"version"`
	AcadVer    string         `json:"acadver"`
	HeaderVars int            `json:"header_vars"`
	Classes    int            `json:"classes"`
	Tables     []tableJSON    `json:"tables"`
	Blocks     []string       `json:"blocks"`
	Entities   int            `json:"entities"`
	Kinds      map[string]int `json:"entity_kinds"`
	Objects    int            `json:"objects"`
	OtherSecs  []string       `json:"other_sections,omitempty"`
}

func main() {
	withEntities := flag.Bool("entities", false, "list every entity record")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: dxfinfo [-entities] <file.dxf>")
		os.Exit(2)
	}
	path := flag.Arg(0)

	doc, err := dxf.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dxfinfo: %v\n", err)
		os.Exit(1)
	}

	s := summaryJSON{
		File:       path,
		Format:     "dxf",
		Version:    doc.Version.String(),
		AcadVer:    doc.Version.AcadVer(),
		HeaderVars: len(doc.Header),
		Classes:    len(doc.Classes),
		Blocks:     []string{},
		Tables:     []tableJSON{},
		Kinds:      map[string]int{},
	}
	for _, t := range doc.Tables {
		tj := tableJSON{Name: t.Name}
		for _, e := range t.Entries {
			tj.Entries = append(tj.Entries, e.Name())
		}
		s.Tables = append(s.Tables, tj)
	}
	for _, b := range doc.Blocks {
		s.Blocks = append(s.Blocks, b.Name())
	}
	s.Entities = len(doc.Entities)
	for _, e := range doc.Entities {
		s.Kinds[e.Type]++
		if *withEntities {
			fmt.Printf("%s\n", e.Type)
		}
	}
	s.Objects = len(doc.Objects)
	for _, sec := range doc.OtherSections {
		s.OtherSecs = append(s.OtherSecs, sec.Name)
	}

	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "dxfinfo: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
