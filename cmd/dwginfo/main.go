// Command dwginfo prints a JSON summary of a DWG file: version, section
// inventory, metadata, handle map and object-walk counts.
//
// Usage:
//
//	dwginfo [-objects] [-max-objects N] <file.dwg>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/flywave/go-dwg/dwg"
)

type sectionJSON struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Offset     uint64 `json:"offset"`
	Size       uint64 `json:"size"`
	Compressed bool   `json:"compressed"`
	Encrypted  bool   `json:"encrypted"`
}

type objectJSON struct {
	Offset int    `json:"offset"`
	Size   uint32 `json:"size"`
	Type   uint16 `json:"type"`
	Label  string `json:"label"`
	Handle uint64 `json:"handle"`
	RawLen int    `json:"raw_len"`
}

type classJSON struct {
	Number      uint16 `json:"number"`
	App         string `json:"app"`
	Cpp         string `json:"cpp"`
	Dxf         string `json:"dxf"`
	NumObjects  uint32 `json:"num_objects"`
	DwgVersion  uint32 `json:"dwg_version"`
	Maintenance uint32 `json:"maintenance_version"`
}

type entityJSON struct {
	Label      string      `json:"label"`
	Type       uint16      `json:"type"`
	Start      *[3]float64 `json:"start,omitempty"`
	End        *[3]float64 `json:"end,omitempty"`
	Center     *[3]float64 `json:"center,omitempty"`
	Position   *[3]float64 `json:"position,omitempty"`
	Radius     *float64    `json:"radius,omitempty"`
	Thickness  *float64    `json:"thickness,omitempty"`
	StartAngle *float64    `json:"start_angle,omitempty"`
	EndAngle   *float64    `json:"end_angle,omitempty"`
	Angle      *float64    `json:"angle,omitempty"`
}

type entitySummaryJSON struct {
	Decoded   int `json:"decoded"`
	Unhandled int `json:"unhandled"`
	Errored   int `json:"errored"`
}

func pt3(p dwg.Point3D) *[3]float64 { return &[3]float64{p.X, p.Y, p.Z} }

type info struct {
	Magic            string             `json:"magic"`
	Version          string             `json:"version"`
	FileSize         int                `json:"file_size"`
	SectionMapStatus string             `json:"section_map_status"`
	Sections         []sectionJSON      `json:"sections"`
	HandleEntries    *int               `json:"handle_entries,omitempty"`
	ClassMax         *uint32            `json:"class_max,omitempty"`
	Classes          []classJSON        `json:"classes,omitempty"`
	ObjectsSeqCount  *int               `json:"objects_seq_count,omitempty"`
	AllObjectsCount  *int               `json:"all_objects_count,omitempty"`
	Objects          []objectJSON       `json:"objects,omitempty"`
	Entities         []entityJSON       `json:"entities,omitempty"`
	EntitySummary    *entitySummaryJSON `json:"entity_summary,omitempty"`
	Title            string             `json:"title,omitempty"`
	Author           string             `json:"author,omitempty"`
	AppName          string             `json:"app_name,omitempty"`
	AppVersion       string             `json:"app_version,omitempty"`
	Warning          string             `json:"warning,omitempty"`
}

func statusName(s dwg.SectionMapStatus) string {
	switch s.State {
	case dwg.StatusFull:
		return "Full"
	case dwg.StatusFallback:
		return "Fallback"
	default:
		return "Deferred"
	}
}

func main() {
	showObjects := flag.Bool("objects", false, "include the handle-driven object list")
	showEntities := flag.Bool("entities", false, "include decoded LINE/CIRCLE/ARC/POINT geometry")
	maxObjects := flag.Int("max-objects", 200, "maximum objects to include")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: dwginfo [-objects] [-entities] [-max-objects N] <file.dwg>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	bytes, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	gf, err := dwg.DwgFromBytes(bytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	out := info{
		Magic:            gf.Version().Magic(),
		Version:          gf.Version().Release(),
		FileSize:         len(bytes),
		SectionMapStatus: statusName(gf.SectionMapStatus()),
	}
	for _, s := range gf.Sections() {
		out.Sections = append(out.Sections, sectionJSON{
			Name: s.Name, Kind: s.Kind.ShortLabel(), Offset: s.Offset, Size: s.Size,
			Compressed: s.Compressed, Encrypted: s.Encrypted,
		})
	}
	if hm, ok, err := gf.HandleMap(); ok && err == nil {
		n := hm.Len()
		out.HandleEntries = &n
	}
	if cm, ok, err := gf.ClassMap(); ok && err == nil {
		m := cm.MaxClassNumber
		out.ClassMax = &m
		for _, c := range cm.Classes {
			out.Classes = append(out.Classes, classJSON{
				Number: c.ClassNumber, App: c.AppName, Cpp: c.CppClassName, Dxf: c.DxfClassName,
				NumObjects: c.NumObjects, DwgVersion: c.DwgVersion, Maintenance: c.MaintenanceVersion,
			})
		}
	}
	if objs, ok, err := gf.Objects(); ok && err == nil {
		n := len(objs)
		out.ObjectsSeqCount = &n
	}
	if objs, _, ok, err := gf.AllObjectsLossy(); ok && err == nil {
		n := len(objs)
		out.AllObjectsCount = &n
		if *showObjects {
			limit := *maxObjects
			if limit > len(objs) {
				limit = len(objs)
			}
			for _, o := range objs[:limit] {
				out.Objects = append(out.Objects, objectJSON{
					Offset: o.StreamOffset, Size: o.SizeBytes, Type: o.TypeCode,
					Label: o.Kind.String(), Handle: o.Handle.Value, RawLen: len(o.Raw),
				})
			}
		}
	}
	if si, ok, err := gf.SummaryInfo(); ok && err == nil {
		out.Title = si.Title
		out.Author = si.Author
	}
	if *showEntities {
		if ents, summary, ok, err := gf.DecodeAllEntities(); ok && err == nil {
			out.EntitySummary = &entitySummaryJSON{Decoded: summary.Decoded, Unhandled: summary.Unhandled, Errored: summary.Errored}
			for _, e := range ents {
				ej := entityJSON{Label: e.Label, Type: e.TypeCode}
				switch v := e.Value.(type) {
				case *dwg.Line:
					ej.Start, ej.End = pt3(v.Start), pt3(v.End)
					ej.Thickness = &v.Thickness
				case *dwg.Circle:
					ej.Center, ej.Radius, ej.Thickness = pt3(v.Center), &v.Radius, &v.Thickness
				case *dwg.Arc:
					ej.Center, ej.Radius, ej.Thickness = pt3(v.Center), &v.Radius, &v.Thickness
					ej.StartAngle, ej.EndAngle = &v.StartAngle, &v.EndAngle
				case *dwg.Point:
					ej.Position, ej.Thickness, ej.Angle = pt3(v.Position), &v.Thickness, &v.XAxisAngle
				}
				if e.Value != nil {
					out.Entities = append(out.Entities, ej)
				}
			}
		}
	}
	if ai, ok, err := gf.AppInfo(); ok && err == nil {
		out.AppName = ai.Name
		out.AppVersion = ai.Version
	}
	if gf.SectionMapStatus().State != dwg.StatusFull {
		out.Warning = gf.SectionMapStatus().Reason
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
