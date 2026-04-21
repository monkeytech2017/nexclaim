package tests

import (
	"testing"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/router"
)

func TestRoute_CSMBS_OPD(t *testing.T) {
	r, err := router.Route(model.INSCL_CSMBS, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Format != model.FormatCSOP {
		t.Errorf("want FormatCSOP, got %s", r.Format)
	}
	if r.Agency != model.AgencyCGD {
		t.Errorf("want AgencyCGD, got %s", r.Agency)
	}
	if r.Sender != model.SenderFDH {
		t.Errorf("want SenderFDH, got %s", r.Sender)
	}
}

func TestRoute_CSMBS_IPD(t *testing.T) {
	r, err := router.Route(model.INSCL_CSMBS, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Format != model.FormatCIPN {
		t.Errorf("want FormatCIPN, got %s", r.Format)
	}
}

func TestRoute_SSO_OPD(t *testing.T) {
	r, _ := router.Route(model.INSCL_SSS, false)
	if r.Format != model.FormatSSOP || r.Sender != model.SenderCHI {
		t.Errorf("got format=%s sender=%s", r.Format, r.Sender)
	}
}

func TestRoute_SSO_IPD(t *testing.T) {
	r, _ := router.Route(model.INSCL_SSS, true)
	if r.Format != model.FormatAIPN || r.Sender != model.SenderCHI {
		t.Errorf("got format=%s sender=%s", r.Format, r.Sender)
	}
}

func TestRoute_UC(t *testing.T) {
	r, _ := router.Route(model.INSCL_UCS, false)
	if r.Format != model.Format16Files || r.Sender != model.SenderFDH {
		t.Errorf("got format=%s sender=%s", r.Format, r.Sender)
	}
}

func TestRoute_WK(t *testing.T) {
	r, _ := router.Route(model.INSCL_WK, false)
	if r.Sender != model.SenderWCF {
		t.Errorf("want SenderWCF, got %s", r.Sender)
	}
}

func TestRoute_Unknown_Fallback(t *testing.T) {
	r, err := router.Route("UNKNOWN", false)
	if err == nil {
		t.Error("expected error for unknown INSCL")
	}
	if r.Format != model.Format16Files {
		t.Errorf("fallback should be Format16Files, got %s", r.Format)
	}
}
