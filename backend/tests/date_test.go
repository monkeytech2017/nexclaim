package tests

import (
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/util"
)

func TestToAD_BE(t *testing.T) {
	got := util.ToAD("25680421") // พ.ศ. 2568
	want := "20250421"
	if got != want {
		t.Errorf("ToAD(25680421) = %s, want %s", got, want)
	}
}

func TestToAD_CE(t *testing.T) {
	got := util.ToAD("20250421")
	if got != "20250421" {
		t.Errorf("ToAD(20250421) should not change, got %s", got)
	}
}

func TestToAD_Short(t *testing.T) {
	got := util.ToAD("123")
	if got != "123" {
		t.Errorf("ToAD(123) should return 123, got %s", got)
	}
}

func TestFormatDate(t *testing.T) {
	tm, _ := time.Parse("2006-01-02", "2025-04-21")
	got := util.FormatDate(tm)
	if got != "20250421" {
		t.Errorf("FormatDate = %s, want 20250421", got)
	}
}

func TestParseHISDate_CE(t *testing.T) {
	tm, err := util.ParseHISDate("20250421")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.Year() != 2025 {
		t.Errorf("year = %d, want 2025", tm.Year())
	}
}

func TestParseHISDate_BE(t *testing.T) {
	tm, err := util.ParseHISDate("25680421")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.Year() != 2025 {
		t.Errorf("year = %d, want 2025", tm.Year())
	}
}

func TestParseHISDate_Invalid(t *testing.T) {
	_, err := util.ParseHISDate("notadate")
	if err == nil {
		t.Error("expected error for invalid date")
	}
}
