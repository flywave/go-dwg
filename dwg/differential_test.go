package dwg

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// oracleJSON mirrors the JSON emitted by tools/dwgref.
type oracleJSON struct {
	Magic            string `json:"magic"`
	Version          string `json:"version"`
	FileSize         uint64 `json:"file_size"`
	SectionMapStatus string `json:"section_map_status"`
	Sections         []struct {
		Name       string `json:"name"`
		Kind       string `json:"kind"`
		Offset     uint64 `json:"offset"`
		Size       uint64 `json:"size"`
		Compressed bool   `json:"compressed"`
		Encrypted  bool   `json:"encrypted"`
	} `json:"sections"`
	SectionManifest []struct {
		Name string `json:"name"`
		File string `json:"file"`
		Len  int    `json:"len"`
	} `json:"section_manifest"`
	HandleMapEntries *int        `json:"handle_map_entries"`
	HandleMapFirst   [][2]uint64 `json:"handle_map_first"`
	ClassMax         *uint32     `json:"class_max"`
	Classes          []struct {
		N     uint16 `json:"n"`
		App   string `json:"app"`
		Cpp   string `json:"cpp"`
		Dxf   string `json:"dxf"`
		Num   uint32 `json:"num"`
		Dwg   uint32 `json:"dwg"`
		Maint uint32 `json:"maint"`
		Proxy bool   `json:"proxy"`
		Item  uint16 `json:"item"`
	} `json:"classes"`
	HeaderVars *struct {
		SentinelOK bool   `json:"sentinel_ok"`
		SizeInBits uint32 `json:"size_in_bits"`
		BodyLen    int    `json:"body_len"`
	} `json:"header_vars"`
	ObjectsSeqCount *int `json:"objects_seq_count"`
	AllObjectsCount *int `json:"all_objects_count"`
	Walk            *struct {
		Decoded      int `json:"decoded"`
		Skipped      int `json:"skipped"`
		UnknownTypes int `json:"unknown_types"`
		Mismatches   int `json:"mismatches"`
	} `json:"walk"`
	AllObjects []struct {
		Off    int    `json:"off"`
		Size   uint32 `json:"size"`
		Type   uint16 `json:"type"`
		Label  string `json:"label"`
		Handle uint64 `json:"handle"`
		RawLen int    `json:"raw_len"`
	} `json:"all_objects"`
	SummaryInfo *struct {
		Title       string `json:"title"`
		Author      string `json:"author"`
		Subject     string `json:"subject"`
		Keywords    string `json:"keywords"`
		LastSavedBy string `json:"last_saved_by"`
		Props       int    `json:"props"`
	} `json:"summary_info"`
	AppInfo *struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Product string `json:"product"`
	} `json:"app_info"`
	DecodedEntities *struct {
		Decoded   int `json:"decoded"`
		Unhandled int `json:"unhandled"`
		Errored   int `json:"errored"`
	} `json:"decoded_entities"`
	EntityValues []entValue `json:"entity_values"`
}

// entValue mirrors the oracle's per-entity field projection. Floats are
// exact IEEE-754 bit patterns.
type entValue struct {
	T          string   `json:"t"`
	Is2D       *bool    `json:"is2d,omitempty"`
	Start      []uint64 `json:"start,omitempty"`
	End        []uint64 `json:"end,omitempty"`
	Center     []uint64 `json:"center,omitempty"`
	Position   []uint64 `json:"position,omitempty"`
	Extrusion  []uint64 `json:"extrusion,omitempty"`
	Radius     *uint64  `json:"radius,omitempty"`
	Thickness  *uint64  `json:"thickness,omitempty"`
	Angle      *uint64  `json:"angle,omitempty"`
	StartAngle *uint64  `json:"start_angle,omitempty"`
	EndAngle   *uint64  `json:"end_angle,omitempty"`
	// Non-entity objects.
	Cloning    *int16   `json:"cloning,omitempty"`
	Hard       *bool    `json:"hard,omitempty"`
	Keys       []string `json:"keys,omitempty"`
	Reactors   *uint32  `json:"reactors,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	N          *uint32  `json:"n,omitempty"`
	DimRC      *uint8   `json:"dimrc,omitempty"`
	Data       string   `json:"data,omitempty"`
	Name       string   `json:"name,omitempty"`
	Flags      *int16   `json:"flags,omitempty"`
	Plot       *bool    `json:"plot,omitempty"`
	Lw         *int16   `json:"lw,omitempty"`
	Color      *int16   `json:"color,omitempty"`
	Tab        *int32   `json:"tab,omitempty"`
	Desc       string   `json:"desc,omitempty"`
	Pattern    *uint64  `json:"pattern,omitempty"`
	Align      *uint8   `json:"align,omitempty"`
	Nd         *int     `json:"nd,omitempty"`
	Fixed      *uint64  `json:"fixed,omitempty"`
	Font       string   `json:"font,omitempty"`
	Bigfont    string   `json:"bigfont,omitempty"`
	Anon       *bool    `json:"anon,omitempty"`
	IsXref     *bool    `json:"isxref,omitempty"`
	Owned      *uint32  `json:"owned,omitempty"`
	Base       []uint64 `json:"base,omitempty"`
	Xref       string   `json:"xref,omitempty"`
	Sentinel   *uint64  `json:"sentinel,omitempty"`
	Dimscale   *uint64  `json:"dimscale,omitempty"`
	Dimasz     *uint64  `json:"dimasz,omitempty"`
	Dimexo     *uint64  `json:"dimexo,omitempty"`
	Dimexe     *uint64  `json:"dimexe,omitempty"`
	Dimtxt     *uint64  `json:"dimtxt,omitempty"`
	Dimcen     *uint64  `json:"dimcen,omitempty"`
	Dimtfac    *uint64  `json:"dimtfac,omitempty"`
	Dimlfac    *uint64  `json:"dimlfac,omitempty"`
	Dimaltf    *uint64  `json:"dimaltf,omitempty"`
	Dimaltrnd  *uint64  `json:"dimaltrnd,omitempty"`
	Dimtad     *uint8   `json:"dimtad,omitempty"`
	Dimtolj    *uint8   `json:"dimtolj,omitempty"`
	Dimtih     *bool    `json:"dimtih,omitempty"`
	Dimtoh     *bool    `json:"dimtoh,omitempty"`
	Dimupt     *bool    `json:"dimupt,omitempty"`
	Schema     *uint8   `json:"schema,omitempty"`
	Value      string   `json:"value,omitempty"`
	Version    *int16   `json:"version,omitempty"`
	Paper      *uint64  `json:"paper,omitempty"`
	Drawing    *uint64  `json:"drawing,omitempty"`
	Unit       *bool    `json:"unit,omitempty"`
	Ist        *int16   `json:"ist,omitempty"`
	Fmt        *int16   `json:"fmt,omitempty"`
	Internal   *bool    `json:"internal,omitempty"`
	Flm        *int16   `json:"flm,omitempty"`
	Flq        *int16   `json:"flq,omitempty"`
	Fcm        *int16   `json:"fcm,omitempty"`
	Fmod       *int16   `json:"fmod,omitempty"`
	Fop        *uint64  `json:"fop,omitempty"`
	Fsp        *uint64  `json:"fsp,omitempty"`
	Emod       *int16   `json:"emod,omitempty"`
	Est        *int16   `json:"est,omitempty"`
	Ecr        *uint64  `json:"ecr,omitempty"`
	Dw         *int16   `json:"dw,omitempty"`
	Ov         *int16   `json:"ov,omitempty"`
	Jt         *int16   `json:"jt,omitempty"`
	Sw         *int16   `json:"sw,omitempty"`
	Hg         *int16   `json:"hg,omitempty"`
	Ic         *int16   `json:"ic,omitempty"`
	Hp         *bool    `json:"hp,omitempty"`
	Db         *uint64  `json:"db,omitempty"`
	Dst        *int16   `json:"dst,omitempty"`
	Nflags     *int     `json:"nflags,omitempty"`
	Next       *int     `json:"next,omitempty"`
	Nstr       *int     `json:"nstr,omitempty"`
	Content    *int16   `json:"content,omitempty"`
	Dmo        *int16   `json:"dmo,omitempty"`
	Dlo        *int16   `json:"dlo,omitempty"`
	Maxseg     *int32   `json:"maxseg,omitempty"`
	Fsa        *uint64  `json:"fsa,omitempty"`
	Ssa        *uint64  `json:"ssa,omitempty"`
	Llt        *int16   `json:"llt,omitempty"`
	Llw        *int32   `json:"llw,omitempty"`
	Landing    *bool    `json:"landing,omitempty"`
	Lgb        *uint64  `json:"lgb,omitempty"`
	Dogleg     *bool    `json:"dogleg,omitempty"`
	Dl         *uint64  `json:"dl,omitempty"`
	Arrow      *uint64  `json:"arrow,omitempty"`
	Tla        *int16   `json:"tla,omitempty"`
	Tra        *int16   `json:"tra,omitempty"`
	Tat        *int16   `json:"tat,omitempty"`
	Talt       *int16   `json:"talt,omitempty"`
	Th         *uint64  `json:"th,omitempty"`
	Frame      *bool    `json:"frame,omitempty"`
	Alwaysleft *bool    `json:"alwaysleft,omitempty"`
	Aspace     *uint64  `json:"aspace,omitempty"`
	Scale      *uint64  `json:"scale,omitempty"`
	Overwrite  *bool    `json:"overwrite,omitempty"`
	Annot      *bool    `json:"annot,omitempty"`
	Bg         *uint64  `json:"bg,omitempty"`
	Tad        *int16   `json:"tad,omitempty"`
	Btad       *int16   `json:"btad,omitempty"`
	Ttad       *int16   `json:"ttad,omitempty"`
	Ext        *bool    `json:"ext,omitempty"`
	Empty      *bool    `json:"empty,omitempty"`
	Instore    *bool    `json:"instore,omitempty"`
	Wf         *bool    `json:"wf,omitempty"`
	Pt         []uint64 `json:"pt,omitempty"`
	Iso        *int32   `json:"iso,omitempty"`
	Isop       *bool    `json:"isop,omitempty"`
	Nw         *int32   `json:"nw,omitempty"`
	Ns         *int32   `json:"ns,omitempty"`
	Ae2        *bool    `json:"ae2,omitempty"`
	Guid       string   `json:"guid,omitempty"`
	SBColor    *uint32  `json:"sbcolor,omitempty"`
	// VPORT.
	Vh     *uint64  `json:"vh,omitempty"`
	Vw     *uint64  `json:"vw,omitempty"`
	Vc     []uint64 `json:"vc,omitempty"`
	Vt     []uint64 `json:"vt,omitempty"`
	Vd     []uint64 `json:"vd,omitempty"`
	Twist  *uint64  `json:"twist,omitempty"`
	Lens   *uint64  `json:"lens,omitempty"`
	Fclip  *uint64  `json:"fclip,omitempty"`
	Bclip  *uint64  `json:"bclip,omitempty"`
	Vmode  *int16   `json:"vmode,omitempty"`
	Rmode  *uint8   `json:"rmode,omitempty"`
	Ll     []uint64 `json:"ll,omitempty"`
	Ur     []uint64 `json:"ur,omitempty"`
	Ucsorg *bool    `json:"ucsorg,omitempty"`
	Ucsper *bool    `json:"ucsper,omitempty"`
	Sbase  []uint64 `json:"sbase,omitempty"`
	Ssp    []uint64 `json:"ssp,omitempty"`
	Gsp    []uint64 `json:"gsp,omitempty"`
	Srot   *uint64  `json:"srot,omitempty"`
	// SECTIONVIEWSTYLE — a 34-field record, projected as ordered arrays
	// (svi ints, svb flag bits, svf float bit-patterns, svc colours as
	// index/rgb pairs, svs strings) in wire order.
	SVName  string   `json:"svname,omitempty"`
	SVAlias string   `json:"svalias,omitempty"`
	SVI     []int64  `json:"svi,omitempty"`
	SVB     []bool   `json:"svb,omitempty"`
	SVF     []uint64 `json:"svf,omitempty"`
	SVC     []uint64 `json:"svc,omitempty"`
	SVS     []string `json:"svs,omitempty"`
	SVHF    *uint8   `json:"svhf,omitempty"`
	// HATCH — a canonical signature string, rendered identically by the
	// oracle (see dwg/hatchsig.go).
	Hatch string `json:"hatch,omitempty"`
	// ELLIPSE.
	Emajor []uint64 `json:"emajor,omitempty"`
	Eaxis  *uint64  `json:"eaxis,omitempty"`
	Estart *uint64  `json:"estart,omitempty"`
	Eend   *uint64  `json:"eend,omitempty"`
	// MLINESTYLE.
	MName  string   `json:"mname,omitempty"`
	MDesc  string   `json:"mdesc,omitempty"`
	MFlags *int16   `json:"mflags,omitempty"`
	MFill  *uint32  `json:"mfill,omitempty"`
	MSa    *uint64  `json:"msa,omitempty"`
	Mea    *uint64  `json:"mea,omitempty"`
	Mlines []uint64 `json:"mlines,omitempty"`
}

// oracleBinary locates (or builds) the dwgref differential oracle.
func oracleBinary(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Skipf("cannot resolve repo root: %v", err)
	}
	bin := filepath.Join(root, "tools", "dwgref", "target", "debug", "dwgref")
	if _, err := os.Stat(bin); err == nil {
		return bin
	}
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		t.Skip("dwgref oracle not built and cargo unavailable; skipping differential test")
	}
	cmd := exec.Command(cargo, "build")
	cmd.Dir = filepath.Join(root, "tools", "dwgref")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cargo build failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("oracle still missing after build: %v", err)
	}
	return bin
}

func runOracle(t *testing.T, bin, file, dumpDir string) (oracleJSON, []byte) {
	t.Helper()
	args := []string{file}
	if dumpDir != "" {
		args = append(args, "--dump-sections", dumpDir)
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		t.Fatalf("oracle failed: %v", err)
	}
	var oj oracleJSON
	if err := json.Unmarshal(out, &oj); err != nil {
		t.Fatalf("oracle JSON decode: %v\n%s", err, out)
	}
	return oj, out
}

func TestDifferentialAgainstRustOracle(t *testing.T) {
	bin := oracleBinary(t)
	files, _ := filepath.Glob("testdata/*.dwg")
	if extra := os.Getenv("DWG_CORPUS"); extra != "" {
		m, _ := filepath.Glob(filepath.Join(extra, "*.dwg"))
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no testdata/*.dwg fixtures")
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			tmp := t.TempDir()
			refDir := filepath.Join(tmp, "ref")
			oj, _ := runOracle(t, bin, f, refDir)

			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			gf, err := DwgFromBytes(raw)
			if err != nil {
				t.Fatalf("Go parse failed: %v", err)
			}

			if got := gf.Version().Release(); got != oj.Version {
				t.Errorf("version = %q want %q", got, oj.Version)
			}
			if gf.FileSize() != oj.FileSize {
				t.Errorf("file_size = %d want %d", gf.FileSize(), oj.FileSize)
			}
			if got := statusVariantName(gf.SectionMapStatus()); got != oj.SectionMapStatus {
				t.Errorf("section_map_status = %q want %q", got, oj.SectionMapStatus)
			}

			// Sections list.
			gs := gf.Sections()
			if len(gs) != len(oj.Sections) {
				t.Fatalf("section count = %d want %d", len(gs), len(oj.Sections))
			}
			for i := range gs {
				o := oj.Sections[i]
				if gs[i].Name != o.Name || gs[i].Kind.ShortLabel() != o.Kind ||
					gs[i].Offset != o.Offset || gs[i].Size != o.Size ||
					gs[i].Compressed != o.Compressed || gs[i].Encrypted != o.Encrypted {
					t.Errorf("section[%d] = %+v want %+v", i, gs[i], o)
				}
			}

			// Section bytes exact.
			for _, m := range oj.SectionManifest {
				want, err := os.ReadFile(filepath.Join(refDir, m.File))
				if err != nil {
					t.Errorf("read ref section %s: %v", m.Name, err)
					continue
				}
				got, ok, err := gf.ReadSection(m.Name)
				if !ok || err != nil {
					t.Errorf("Go read section %s: ok=%v err=%v", m.Name, ok, err)
					continue
				}
				if !bytes.Equal(got, want) {
					t.Errorf("section %s bytes differ: go %d vs ref %d", m.Name, len(got), len(want))
				}
			}

			// Handle map.
			if oj.HandleMapEntries != nil {
				hm, ok, err := gf.HandleMap()
				if !ok || err != nil {
					t.Errorf("handle map: ok=%v err=%v", ok, err)
				} else {
					if hm.Len() != *oj.HandleMapEntries {
						t.Errorf("handle entries = %d want %d", hm.Len(), *oj.HandleMapEntries)
					}
					n := len(oj.HandleMapFirst)
					if n > hm.Len() {
						n = hm.Len()
					}
					for i := 0; i < n; i++ {
						if hm.Entries[i].Handle != oj.HandleMapFirst[i][0] || hm.Entries[i].Offset != oj.HandleMapFirst[i][1] {
							t.Errorf("handle[%d] = (%d,%d) want (%d,%d)", i,
								hm.Entries[i].Handle, hm.Entries[i].Offset,
								oj.HandleMapFirst[i][0], oj.HandleMapFirst[i][1])
							break
						}
					}
				}
			}

			// Class map.
			if oj.ClassMax != nil {
				cm, ok, err := gf.ClassMap()
				if !ok || err != nil {
					t.Errorf("class map: ok=%v err=%v", ok, err)
				} else {
					if cm.MaxClassNumber != *oj.ClassMax {
						t.Errorf("class max = %d want %d", cm.MaxClassNumber, *oj.ClassMax)
					}
					if len(cm.Classes) != len(oj.Classes) {
						t.Errorf("class count = %d want %d", len(cm.Classes), len(oj.Classes))
					} else {
						for i := range cm.Classes {
							o := oj.Classes[i]
							c := cm.Classes[i]
							if c.ClassNumber != o.N || c.AppName != o.App || c.CppClassName != o.Cpp ||
								c.DxfClassName != o.Dxf || c.NumObjects != o.Num || c.DwgVersion != o.Dwg ||
								c.MaintenanceVersion != o.Maint || c.WasAProxy != o.Proxy || c.ItemClassID != o.Item {
								t.Errorf("class[%d] = %+v want %+v", i, c, o)
								break
							}
						}
					}
				}
			}

			// Header vars.
			if oj.HeaderVars != nil {
				hv, ok, err := gf.HeaderVars()
				if !ok || err != nil {
					t.Errorf("header vars: ok=%v err=%v", ok, err)
				} else {
					if hv.HasValidSentinel() != oj.HeaderVars.SentinelOK ||
						hv.SizeInBits != oj.HeaderVars.SizeInBits ||
						hv.BodyLen() != oj.HeaderVars.BodyLen {
						t.Errorf("header vars = %+v want %+v", hv, oj.HeaderVars)
					}
				}
			}

			// Sequential objects.
			if oj.ObjectsSeqCount != nil {
				objs, ok, err := gf.Objects()
				if !ok || err != nil {
					t.Errorf("objects: ok=%v err=%v", ok, err)
				} else if len(objs) != *oj.ObjectsSeqCount {
					t.Errorf("objects seq count = %d want %d", len(objs), *oj.ObjectsSeqCount)
				}
			}

			// Handle-driven objects.
			if oj.AllObjectsCount != nil {
				objs, summary, ok, err := gf.AllObjectsLossy()
				if !ok || err != nil {
					t.Errorf("all objects: ok=%v err=%v", ok, err)
				} else {
					if len(objs) != *oj.AllObjectsCount {
						t.Errorf("all objects count = %d want %d", len(objs), *oj.AllObjectsCount)
					}
					if oj.Walk != nil {
						if summary.DecodedCount != oj.Walk.Decoded ||
							len(summary.Skipped) != oj.Walk.Skipped ||
							len(summary.UnknownTypes) != oj.Walk.UnknownTypes ||
							len(summary.HandleMismatches) != oj.Walk.Mismatches {
							t.Errorf("walk summary = %+v want %+v", summary, oj.Walk)
						}
					}
					if len(objs) == len(oj.AllObjects) {
						for i := range objs {
							o := oj.AllObjects[i]
							if objs[i].StreamOffset != o.Off || objs[i].SizeBytes != o.Size ||
								objs[i].TypeCode != o.Type || objs[i].Kind.String() != o.Label ||
								objs[i].Handle.Value != o.Handle || len(objs[i].Raw) != o.RawLen {
								t.Errorf("object[%d] = {off:%d size:%d type:%d label:%s handle:%d raw:%d} want %+v",
									i, objs[i].StreamOffset, objs[i].SizeBytes, objs[i].TypeCode,
									objs[i].Kind.String(), objs[i].Handle.Value, len(objs[i].Raw), o)
								break
							}
						}
					}
				}
			}

			// Summary info.
			if oj.SummaryInfo != nil {
				si, ok, err := gf.SummaryInfo()
				if !ok || err != nil {
					t.Errorf("summary info: ok=%v err=%v", ok, err)
				} else {
					o := oj.SummaryInfo
					if si.Title != o.Title || si.Author != o.Author || si.Subject != o.Subject ||
						si.Keywords != o.Keywords || si.LastSavedBy != o.LastSavedBy || len(si.Properties) != o.Props {
						t.Errorf("summary info = %+v want %+v", si, o)
					}
				}
			}

			// App info.
			if oj.AppInfo != nil {
				ai, ok, err := gf.AppInfo()
				if !ok || err != nil {
					t.Errorf("app info: ok=%v err=%v", ok, err)
				} else {
					o := oj.AppInfo
					if ai.Name != o.Name || ai.Version != o.Version || ai.Product != o.Product {
						t.Errorf("app info = %+v want %+v", ai, o)
					}
				}
			}

			// Decoded entity field values (LINE/CIRCLE/ARC/POINT).
			if oj.DecodedEntities != nil {
				ents, summary, ok, err := gf.DecodeAllEntities()
				if !ok || err != nil {
					t.Errorf("decode entities: ok=%v err=%v", ok, err)
				} else {
					// The oracle resolves custom classes through the class
					// map; this port does not, so only compare the summary
					// when no class map is present.
					if oj.ClassMax == nil {
						o := oj.DecodedEntities
						if summary.Decoded != o.Decoded || summary.Unhandled != o.Unhandled || summary.Errored != o.Errored {
							t.Errorf("dispatch summary = %+v want %+v", summary, o)
						}
					}
					got := goEntityValues(ents)
					if !subsequenceMatch(got, oj.EntityValues) {
						t.Errorf("entity values differ:\n go  = %s\n ref = %s",
							jsonDebug(got), jsonDebug(oj.EntityValues))
					}
				}
			}

			_ = reflect.DeepEqual
		})
	}
}

func u64p(v uint64) *uint64 { return &v }
func u32p(v uint32) *uint32 { return &v }
func u8p(v uint8) *uint8    { return &v }
func i16p(v int16) *int16   { return &v }
func boolp(v bool) *bool    { return &v }
func intp(v int) *int       { return &v }
func i32p(v int32) *int32   { return &v }

func ptBits(p Point3D) []uint64 {
	return []uint64{math.Float64bits(p.X), math.Float64bits(p.Y), math.Float64bits(p.Z)}
}

func pt2Bits(p PlotPoint2D) []uint64 {
	return []uint64{math.Float64bits(p.X), math.Float64bits(p.Y)}
}

func pt2Bits3(p PlotPoint3D) []uint64 {
	return []uint64{math.Float64bits(p.X), math.Float64bits(p.Y), math.Float64bits(p.Z)}
}

// goEntityValues projects Go's decoded entities into the oracle's comparison
// shape (only the geometry types it emits).
func goEntityValues(ents []DecodedEntity) []entValue {
	out := []entValue{}
	for _, e := range ents {
		switch v := e.Value.(type) {
		case *Line:
			out = append(out, entValue{T: "LINE", Is2D: boolp(v.Is2D),
				Start: ptBits(v.Start), End: ptBits(v.End),
				Thickness: u64p(math.Float64bits(v.Thickness)), Extrusion: ptBits(v.Extrusion)})
		case *Circle:
			out = append(out, entValue{T: "CIRCLE", Center: ptBits(v.Center),
				Radius: u64p(math.Float64bits(v.Radius)), Thickness: u64p(math.Float64bits(v.Thickness)),
				Extrusion: ptBits(v.Extrusion)})
		case *Arc:
			out = append(out, entValue{T: "ARC", Center: ptBits(v.Center),
				Radius: u64p(math.Float64bits(v.Radius)), Thickness: u64p(math.Float64bits(v.Thickness)),
				Extrusion: ptBits(v.Extrusion), StartAngle: u64p(math.Float64bits(v.StartAngle)),
				EndAngle: u64p(math.Float64bits(v.EndAngle))})
		case *Point:
			out = append(out, entValue{T: "POINT", Position: ptBits(v.Position),
				Thickness: u64p(math.Float64bits(v.Thickness)), Extrusion: ptBits(v.Extrusion),
				Angle: u64p(math.Float64bits(v.XAxisAngle))})
		case *Dictionary:
			out = append(out, entValue{T: "DICTIONARY", Cloning: i16p(v.CloningFlag),
				Hard: boolp(v.HardOwner), Keys: v.Keys})
		case *Placeholder:
			out = append(out, entValue{T: "PLACEHOLDER", Reactors: u32p(v.NumReactors)})
		case *XRecord:
			out = append(out, entValue{T: "XRECORD", Cloning: i16p(v.CloningFlags),
				Data: hex.EncodeToString(v.Data)})
		case *AppID:
			out = append(out, entValue{T: "APPID", Name: v.Header.Name})
		case *LayerEntry:
			out = append(out, entValue{T: "LAYER", Name: v.Header.Name, Flags: i16p(v.Flags),
				Plot: boolp(v.PlotFlag), Lw: i16p(v.Lineweight), Color: i16p(v.ColorIndex)})
		case *LtypeEntry:
			nd := len(v.Dashes)
			out = append(out, entValue{T: "LTYPE", Name: v.Header.Name, Desc: v.Description,
				Pattern: u64p(math.Float64bits(v.PatternLength)), Align: u8p(v.Alignment), Nd: &nd})
		case *StyleEntry:
			out = append(out, entValue{T: "STYLE", Name: v.Header.Name, Flags: i16p(int16(v.Flags)),
				Fixed: u64p(math.Float64bits(v.FixedHeight)), Font: v.FontFilename, Bigfont: v.BigfontFilename})
		case *Block:
			out = append(out, entValue{T: "BLOCK", Name: v.Name})
		case *EndBlk:
			out = append(out, entValue{T: "ENDBLK"})
		case *SeqEnd:
			out = append(out, entValue{T: "SEQEND"})
		case *BlockRecord:
			ev := entValue{T: "BLOCK_HEADER", Name: v.Header.Name, Anon: boolp(v.IsAnonymous),
				IsXref: boolp(v.IsXref), Base: ptBits(v.BasePoint), Xref: v.XrefPath, Desc: v.Description}
			if v.HasNumOwned {
				ev.Owned = u32p(v.NumOwnedObjects)
			}
			if v.HasSentinel {
				ev.Sentinel = u64p(v.BlockSentinelHandle)
			}
			out = append(out, ev)
		case *DimStyleEntry:
			out = append(out, entValue{T: "DIMSTYLE", Name: v.Header.Name,
				Dimscale: u64p(math.Float64bits(v.Dimscale)), Dimasz: u64p(math.Float64bits(v.Dimasz)),
				Dimexo: u64p(math.Float64bits(v.Dimexo)), Dimexe: u64p(math.Float64bits(v.Dimexe)),
				Dimtxt: u64p(math.Float64bits(v.Dimtxt)), Dimcen: u64p(math.Float64bits(v.Dimcen)),
				Dimtfac: u64p(math.Float64bits(v.Dimtfac)), Dimlfac: u64p(math.Float64bits(v.Dimlfac)),
				Dimaltf: u64p(math.Float64bits(v.Dimaltf)), Dimaltrnd: u64p(math.Float64bits(v.Dimaltrnd)),
				Dimtad: u8p(v.Dimtad), Dimtolj: u8p(v.Dimtolj),
				Dimtih: boolp(v.Dimtih), Dimtoh: boolp(v.Dimtoh), Dimupt: boolp(v.Dimupt)})
		case *DictionaryVar:
			out = append(out, entValue{T: "DICTIONARYVAR", Schema: u8p(v.Schema), Value: v.Value})
		case *Scale:
			out = append(out, entValue{T: "SCALE", Name: v.ScaleName, Version: i16p(v.Version),
				Paper: u64p(math.Float64bits(v.PaperUnits)), Drawing: u64p(math.Float64bits(v.DrawingUnits)),
				Unit: boolp(v.IsUnitScale)})
		case *VisualStyle:
			out = append(out, entValue{T: "VISUALSTYLE", Name: v.Description,
				Ist: i16p(v.InternalStyleType), Fmt: i16p(v.FormatVersion), Internal: boolp(v.IsInternalUseOnly),
				Flm: i16p(v.FaceLightingModel), Flq: i16p(v.FaceLightingQuality), Fcm: i16p(v.FaceColorMode),
				Fmod: i16p(v.FaceModifier), Fop: u64p(math.Float64bits(v.FaceOpacity)),
				Fsp: u64p(math.Float64bits(v.FaceSpecular)), Emod: i16p(v.EdgeModel), Est: i16p(v.EdgeStyle),
				Ecr: u64p(math.Float64bits(v.EdgeCreaseAngle)), Dw: i16p(v.EdgeWidth), Ov: i16p(v.EdgeOverhang),
				Jt: i16p(v.EdgeJitter), Sw: i16p(v.EdgeSilhouetteWidth), Hg: i16p(v.EdgeHaloGap),
				Ic: i16p(v.EdgeIsolineCount), Hp: boolp(v.EdgeHidePrecision),
				Db: u64p(math.Float64bits(v.DisplayBrightness)), Dst: i16p(v.DisplayShadowType),
				Nflags: intp(len(v.PropertyFlags)), Next: intp(v.ExtendedCount), Nstr: intp(len(v.TrailingStrings))})
		case *MLeaderStyle:
			out = append(out, entValue{T: "MLEADERSTYLE", Name: v.Description, Version: i16p(v.Version),
				Content: i16p(v.ContentType), Dmo: i16p(v.DrawMLeaderOrderType), Dlo: i16p(v.DrawLeaderOrderType),
				Maxseg: i32p(v.MaxLeaderSegmentPoints), Fsa: u64p(math.Float64bits(v.FirstSegmentAngle)),
				Ssa: u64p(math.Float64bits(v.SecondSegmentAngle)), Llt: i16p(v.LeaderLineType),
				Llw: i32p(v.LeaderLineweight), Landing: boolp(v.EnableLanding),
				Lgb: u64p(math.Float64bits(v.LandingGap)), Dogleg: boolp(v.EnableDogleg),
				Dl: u64p(math.Float64bits(v.DoglegLength)), Arrow: u64p(math.Float64bits(v.ArrowheadSize)),
				Tla: i16p(v.TextLeftAttachment), Tra: i16p(v.TextRightAttachment),
				Tat: i16p(v.TextAngleType), Talt: i16p(v.TextAlignmentType),
				Th: u64p(math.Float64bits(v.TextHeight)), Frame: boolp(v.EnableFrameText),
				Alwaysleft: boolp(v.TextAlignAlwaysLeft), Aspace: u64p(math.Float64bits(v.AlignSpace)),
				Scale: u64p(math.Float64bits(v.Scale)), Overwrite: boolp(v.OverwritePropertyValue),
				Annot: boolp(v.IsAnnotative), Bg: u64p(math.Float64bits(v.BreakGapSize)),
				Tad: i16p(v.TextAttachmentDirection), Btad: i16p(v.BottomTextAttachmentDirection),
				Ttad: i16p(v.TopTextAttachmentDirection), Ext: boolp(v.ExtendedFlag)})
		case *ModelerRecord:
			ev := entValue{T: e.Label, Empty: boolp(v.Blob.Empty), Instore: boolp(v.InDataStore),
				Wf: boolp(v.Tail.WireframePresent), Iso: i32p(v.Tail.NumIsolines),
				Isop: boolp(v.Tail.IsolinesPresent), Nw: i32p(v.Tail.NumWires),
				Ns: i32p(v.Tail.NumSilhouettes), Ae2: boolp(v.Tail.AcisEmpty2)}
			if v.Tail.Point != nil {
				ev.Pt = ptBits(*v.Tail.Point)
			}
			if v.Tail.RevisionGUID != nil {
				ev.Guid = hex.EncodeToString(v.Tail.RevisionGUID)
			}
			out = append(out, ev)
		case *SolidBackground:
			out = append(out, entValue{T: "SOLID_BACKGROUND", SBColor: u32p(v.Color)})
		case *Sun:
			out = append(out, entValue{T: "SUN", Color: i16p(v.ColorIndex)})
		case *Light:
			out = append(out, entValue{T: "LIGHT", Name: v.Name})
		case *AcadLayout:
			out = append(out, entValue{T: "LAYOUT", Name: v.LayoutName, Tab: i32p(v.TabOrder)})
		case *AcadPlotSettings:
			out = append(out, entValue{T: "PLOTSETTINGS", Name: v.PrinterConfigName})
		case *SortentsTable:
			out = append(out, entValue{T: "SORTENTSTABLE", N: u32p(uint32(v.Count))})
		case *Hatch:
			out = append(out, entValue{T: "HATCH", Hatch: hatchSignature(v)})
		case *Ellipse:
			out = append(out, entValue{T: "ELLIPSE", Center: pt2Bits3(v.Center), Emajor: pt2Bits3(v.MajorAxis),
				Extrusion: pt2Bits3(v.Extrusion), Eaxis: u64p(math.Float64bits(v.AxisRatio)),
				Estart: u64p(math.Float64bits(v.StartParam)), Eend: u64p(math.Float64bits(v.EndParam))})
		case *AcadMlinestyle:
			mlines := []uint64{}
			for _, ln := range v.Lines {
				mlines = append(mlines, math.Float64bits(ln.Offset), uint64(ln.Color.RGB), uint64(uint16(ln.LinetypeIndex)))
			}
			out = append(out, entValue{T: "MLINESTYLE", MName: v.Name, MDesc: v.Description,
				MFlags: i16p(v.Flags), MFill: u32p(v.FillColor.RGB),
				MSa: u64p(math.Float64bits(v.StartAngle)), Mea: u64p(math.Float64bits(v.EndAngle)),
				Mlines: mlines})
		case *AcadSectionViewStyle:
			col := func(c ObjectColor) []uint64 {
				return []uint64{uint64(c.Index), uint64(c.RGB)}
			}
			svc := []uint64{}
			svc = append(svc, col(v.IdentifierColor)...)
			svc = append(svc, col(v.ArrowSymbolColor)...)
			svc = append(svc, col(v.CuttingPlaneLineColor)...)
			svc = append(svc, col(v.EndLineColor)...)
			svc = append(svc, col(v.ViewLabelColor)...)
			svc = append(svc, col(v.HatchColor)...)
			svc = append(svc, col(v.HatchBackgroundColor)...)
			svf := []uint64{
				math.Float64bits(v.IdentifierHeight), math.Float64bits(v.ArrowSymbolSize),
				math.Float64bits(v.IdentifierOffset), math.Float64bits(v.EndLineLength),
				math.Float64bits(v.EndLineOvershoot), math.Float64bits(v.ViewLabelTextHeight),
				math.Float64bits(v.ViewLabelOffset), math.Float64bits(v.HatchScale),
				math.Float64bits(v.HatchSpacing),
			}
			for _, a := range v.HatchAngles {
				svf = append(svf, math.Float64bits(a))
			}
			out = append(out, entValue{T: "SECTIONVIEWSTYLE", SVName: v.Name, SVAlias: v.NameAlias,
				SVI: []int64{int64(v.UnknownHead), int64(v.Flags), int64(v.CuttingPlaneLineWeight),
					int64(v.EndLineWeight), int64(v.ViewLabelPosition), int64(v.ViewLabelAttachment),
					int64(v.HatchTransparency), int64(v.HatchAngleCount), int64(v.UnknownTail)},
				SVB: []bool{v.FlagC, v.FlagD, v.FlagE},
				SVF: svf, SVC: svc,
				SVS:  []string{v.IdentifierExcludeCharacters, v.ViewLabelPattern, v.HatchPatternName},
				SVHF: u8p(v.HatchFlags)})
		case *VportEntry:
			out = append(out, entValue{T: "VPORT", Name: v.Header.Name,
				Vh: u64p(math.Float64bits(v.ViewHeight)), Vw: u64p(math.Float64bits(v.AspectRatio)),
				Vc: pt2Bits(v.ViewCenter), Vt: pt2Bits3(v.ViewTarget), Vd: pt2Bits3(v.ViewDirection),
				Twist: u64p(math.Float64bits(v.ViewTwist)), Lens: u64p(math.Float64bits(v.LensLength)),
				Fclip: u64p(math.Float64bits(v.FrontClip)), Bclip: u64p(math.Float64bits(v.BackClip)),
				Vmode: i16p(v.ViewMode), Rmode: u8p(v.RenderMode),
				Ll: pt2Bits(v.LowerLeft), Ur: pt2Bits(v.UpperRight),
				Ucsorg: boolp(v.UcsAtOrigin), Ucsper: boolp(v.UcsPerVport),
				Sbase: pt2Bits(v.SnapBase), Ssp: pt2Bits(v.SnapSpacing),
				Gsp: pt2Bits(v.GridSpacing), Srot: u64p(math.Float64bits(v.SnapRotation))})
		case *Control:
			ev := entValue{T: "CONTROL", Kind: v.Kind, N: u32p(v.NumEntries)}
			if v.HasDimstyleRC {
				ev.DimRC = u8p(v.DimstyleTrailingRC)
			}
			out = append(out, ev)
		}
	}
	return out
}

func jsonDebug(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Route-B types the reference declines on the corpus but this port
// now decodes (see analysis/gap-fill-plan.md B1.1).
func isRouteBExtra(t string) bool {
	return t == "3DSOLID" || t == "REGION" || t == "BODY" || t == "SOLID_BACKGROUND" || t == "SUN" || t == "LIGHT" || t == "LAYOUT" || t == "PLOTSETTINGS" || t == "SORTENTSTABLE" || t == "PLANESURFACE" || t == "MTEXT" || t == "SH_HISTORY" || t == "EVALUATION_GRAPH" || t == "MENTALRAYRENDERSETTINGS" || t == "MATERIAL"
}

// subsequenceMatch reports whether every oracle entry appears in got in order,
// with any skipped got entries being Route-B modeler extras.
func subsequenceMatch(got, want []entValue) bool {
	gi := 0
	for _, w := range want {
		for gi < len(got) && !reflect.DeepEqual(got[gi], w) {
			if !isRouteBExtra(got[gi].T) {
				return false
			}
			gi++
		}
		if gi >= len(got) {
			return false
		}
		gi++
	}
	for ; gi < len(got); gi++ {
		if !isRouteBExtra(got[gi].T) {
			return false
		}
	}
	return true
}

func statusVariantName(s SectionMapStatus) string {
	switch s.State {
	case StatusFull:
		return "Full"
	case StatusFallback:
		return "Fallback"
	default:
		return "Deferred"
	}
}
