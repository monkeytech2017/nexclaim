package tests

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"testing"

	"github.com/nexclaim/nexclaim/internal/sender"
)

func TestBuildMultiFileZip_ContainsAllFiles(t *testing.T) {
	files := map[string][]byte{
		"A.txt": []byte("alpha"),
		"B.txt": []byte("beta"),
	}
	out, err := sender.BuildMultiFileZip(files)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	r, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	got := map[string][]byte{}
	for _, f := range r.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = b
	}
	for name, content := range files {
		want := append(append([]byte{}, content...), []byte("\n"+fmt.Sprintf("%x", md5.Sum(content)))...)
		if !bytes.Equal(got[name], want) {
			t.Errorf("%s: want %q got %q", name, want, got[name])
		}
	}
}

func TestBuildMultiFileZip_Deterministic(t *testing.T) {
	files := map[string][]byte{
		"A.txt": []byte("alpha"),
		"B.txt": []byte("beta"),
		"C.txt": []byte("gamma"),
	}
	a, _ := sender.BuildMultiFileZip(files)
	b, _ := sender.BuildMultiFileZip(files)
	if !bytes.Equal(a, b) {
		t.Error("zip bytes should be deterministic for equal input")
	}
}

func TestBuildZipWithMD5_HashAppended(t *testing.T) {
	xml := []byte("<CIPN/>")
	out, err := sender.BuildZipWithMD5(xml, "test.xml")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	r, _ := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if len(r.File) != 1 {
		t.Fatalf("want 1 entry, got %d", len(r.File))
	}
	rc, _ := r.File[0].Open()
	b, _ := io.ReadAll(rc)
	rc.Close()
	wantHash := fmt.Sprintf("%x", md5.Sum(xml))
	if !bytes.HasSuffix(b, []byte("\n"+wantHash)) {
		t.Errorf("expected trailing md5 %q, got %q", wantHash, b)
	}
}

func TestZipFilename(t *testing.T) {
	got := sender.ZipFilename("12345", "CIPN", "202504")
	want := "12345CIPN202504.ZIP"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
