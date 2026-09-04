package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatesAtCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tailclip.log")
	writer, err := newRotatingWriter(path, 32)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(strings.Repeat("a", 24))); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(strings.Repeat("b", 24))); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil || string(current) != strings.Repeat("b", 24) {
		t.Fatalf("current=%q err=%v", current, err)
	}
	backup, err := os.ReadFile(path + ".1")
	if err != nil || string(backup) != strings.Repeat("a", 24) {
		t.Fatalf("backup=%q err=%v", backup, err)
	}
}
