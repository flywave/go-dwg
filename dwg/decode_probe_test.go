package dwg

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestDecodeProbeInventory dumps the Go decoder's per-label state across the
// whole corpus (testdata plus DWG_CORPUS) so remaining decode gaps can be
// enumerated. Gated behind DWG_PROBE=1 to keep the default suite quiet.
func TestDecodeProbeInventory(t *testing.T) {
	if os.Getenv("DWG_PROBE") == "" {
		t.Skip("set DWG_PROBE=1 to enumerate decode gaps")
	}
	files, _ := filepath.Glob("testdata/*.dwg")
	if extra := os.Getenv("DWG_CORPUS"); extra != "" {
		m, _ := filepath.Glob(filepath.Join(extra, "*.dwg"))
		files = append(files, m...)
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			gf, err := DwgFromBytes(raw)
			if err != nil {
				t.Fatalf("Go parse failed: %v", err)
			}
			ents, summary, ok, err := gf.DecodeAllEntities()
			if !ok || err != nil {
				t.Logf("DecodeAllEntities unavailable (ok=%v err=%v); skipping", ok, err)
				return
			}
			t.Logf("summary: decoded=%d unhandled=%d errored=%d",
				summary.Decoded, summary.Unhandled, summary.Errored)
			counts := map[string]int{}
			details := map[string][]string{}
			for _, e := range ents {
				counts[e.Label]++
				if e.Label == "UNHANDLED" || e.Label == "ERROR" {
					key := fmt.Sprintf("%s(code=%d)", e.Label, e.TypeCode)
					msg := e.Message
					if len(msg) > 90 {
						msg = msg[:90]
					}
					details[key] = append(details[key], msg)
				}
			}
			labels := make([]string, 0, len(counts))
			for k := range counts {
				labels = append(labels, k)
			}
			sort.Strings(labels)
			for _, k := range labels {
				t.Logf("  %-28s %d", k, counts[k])
			}
			keys := make([]string, 0, len(details))
			for k := range details {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				seen := map[string]bool{}
				for _, m := range details[k] {
					if seen[m] {
						continue
					}
					seen[m] = true
					t.Logf("    %s: %s", k, m)
				}
			}
		})
	}
}
