package tests

import (
	"testing"

	"github.com/nexclaim/nexclaim/internal/hisclient"
	"github.com/nexclaim/nexclaim/internal/sharefile"
)

// fakeResolver maps HIS internal drug code → TMT for tests, mimicking what a
// store.DrugMapRepo-backed factory produces in production.
func fakeResolver(table map[string]string) func(string) (string, bool) {
	return func(hisCode string) (string, bool) {
		tmt, ok := table[hisCode]
		return tmt, ok
	}
}

// ── OPD (hisclient.ToOPDVisit) ──

// (a) HIS-provided TMT wins even when a his_drug_map entry exists.
func TestTMT_OPD_HISProvidedWins(t *testing.T) {
	fix := visitDetailFixture() // drug[0] has TMT24 + TMTTP + HISItemID=DRG000124
	resolver := fakeResolver(map[string]string{
		"DRG000124": "999999999999999999999999", // should be ignored
	})
	v := hisclient.ToOPDVisit(&fix, "HN1", resolver)
	if got := v.Drugs[0].TMTID; got != "100452100101010100000612" {
		t.Errorf("HIS-provided TMT24 must win, got %q", got)
	}

	// And TMTTP wins over the map when TMT24 is empty.
	fix2 := visitDetailFixture()
	fix2.Drug[0].TMT24 = ""
	v2 := hisclient.ToOPDVisit(&fix2, "HN1", resolver)
	if got := v2.Drugs[0].TMTID; got != "1004521" {
		t.Errorf("HIS-provided TMTTP must win over map, got %q", got)
	}
}

// (b) lookup via his_drug_map when both HIS TMT fields are empty.
func TestTMT_OPD_LookupWhenEmpty(t *testing.T) {
	fix := visitDetailFixture()
	fix.Drug[0].TMT24 = ""
	fix.Drug[0].TMTTP = ""
	resolver := fakeResolver(map[string]string{
		"DRG000124": "100452100101010100000612",
	})
	v := hisclient.ToOPDVisit(&fix, "HN1", resolver)
	if got := v.Drugs[0].TMTID; got != "100452100101010100000612" {
		t.Errorf("expected mapped TMT, got %q", got)
	}
}

// (c) no resolver → unchanged fallback (empty when HIS supplies nothing).
func TestTMT_OPD_NoResolverFallback(t *testing.T) {
	fix := visitDetailFixture()
	fix.Drug[0].TMT24 = ""
	fix.Drug[0].TMTTP = ""
	v := hisclient.ToOPDVisit(&fix, "HN1", nil)
	if got := v.Drugs[0].TMTID; got != "" {
		t.Errorf("nil resolver must leave TMT empty, got %q", got)
	}
}

// resolver miss (code not in map) also falls back to empty.
func TestTMT_OPD_ResolverMiss(t *testing.T) {
	fix := visitDetailFixture()
	fix.Drug[0].TMT24 = ""
	fix.Drug[0].TMTTP = ""
	resolver := fakeResolver(map[string]string{"OTHER": "x"})
	v := hisclient.ToOPDVisit(&fix, "HN1", resolver)
	if got := v.Drugs[0].TMTID; got != "" {
		t.Errorf("resolver miss must leave TMT empty, got %q", got)
	}
}

// ── IPD (sharefile.AssembleWithResolver) ──

// druBundle builds a minimal Bundle with one admit + one drug line whose
// tmt_tp/tmt24 are set per the args.
func druBundle(tmtTP, tmt24 string) *sharefile.Bundle {
	return &sharefile.Bundle{
		PAT: []sharefile.PATRow{{PID: "1100101001271", FirstName: "ก", LastName: "ข", DOB: "19850115"}},
		IPD: []sharefile.IPDRow{{
			AN: "AN1", PID: "1100101001271", HN: "HN1", INSCL: "011",
			DateAdm: "20250410", DateDsc: "20250418",
		}},
		DRU: []sharefile.DRURow{{
			AN: "AN1", HISItemID: "DRG000567",
			TMTTP: tmtTP, TMT24: tmt24, Quantity: 1,
		}},
	}
}

// (a) HIS-provided TMT wins for IPD share-file too.
func TestTMT_IPD_HISProvidedWins(t *testing.T) {
	resolver := fakeResolver(map[string]string{"DRG000567": "999999999999999999999999"})
	admits, err := sharefile.AssembleWithResolver(druBundle("1005678", "100567800000000000000099"), resolver)
	if err != nil {
		t.Fatal(err)
	}
	if got := admits[0].Drugs[0].TMTID; got != "100567800000000000000099" {
		t.Errorf("TMT24 must win, got %q", got)
	}

	admits2, _ := sharefile.AssembleWithResolver(druBundle("1005678", ""), resolver)
	if got := admits2[0].Drugs[0].TMTID; got != "1005678" {
		t.Errorf("TMTTP must win over map, got %q", got)
	}
}

// (b) lookup via his_drug_map when empty.
func TestTMT_IPD_LookupWhenEmpty(t *testing.T) {
	resolver := fakeResolver(map[string]string{"DRG000567": "100567800000000000000099"})
	admits, err := sharefile.AssembleWithResolver(druBundle("", ""), resolver)
	if err != nil {
		t.Fatal(err)
	}
	if got := admits[0].Drugs[0].TMTID; got != "100567800000000000000099" {
		t.Errorf("expected mapped TMT, got %q", got)
	}
}

// (c) no resolver → identical to plain Assemble (empty fallback).
func TestTMT_IPD_NoResolverFallback(t *testing.T) {
	admits, err := sharefile.AssembleWithResolver(druBundle("", ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := admits[0].Drugs[0].TMTID; got != "" {
		t.Errorf("nil resolver must leave TMT empty, got %q", got)
	}

	// Assemble (legacy) must equal AssembleWithResolver(b, nil).
	legacy, _ := sharefile.Assemble(druBundle("1005678", ""))
	if got := legacy[0].Drugs[0].TMTID; got != "1005678" {
		t.Errorf("legacy Assemble fallback changed, got %q", got)
	}
}
