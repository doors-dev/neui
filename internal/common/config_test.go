package common

import (
	"context"
	"testing"

	"neui.dev/api"
)

func TestFilesConfigDefaults(t *testing.T) {
	c := FilesConfigDefaults(api.FilesConfig{MaxBytes: -1, ReadLimit: 64})
	if c.MaxBytes != -1 {
		t.Fatalf("MaxBytes = %d, want the configured -1", c.MaxBytes)
	}
	if c.ReadLimit != 64 {
		t.Fatalf("ReadLimit = %d, want the configured 64", c.ReadLimit)
	}
	if c.MaxFiles != 10000 {
		t.Fatalf("MaxFiles = %d, want 10000", c.MaxFiles)
	}
	if c.EditLimit != 10<<20 {
		t.Fatalf("EditLimit = %d, want %d", c.EditLimit, 10<<20)
	}
	if c.ImageFileLimit != 20<<20 {
		t.Fatalf("ImageFileLimit = %d, want %d", c.ImageFileLimit, 20<<20)
	}
	if c.DocumentLimit != 32<<20 {
		t.Fatalf("DocumentLimit = %d, want %d", c.DocumentLimit, 32<<20)
	}
	if c.UnzipLimit != 512<<20 {
		t.Fatalf("UnzipLimit = %d, want %d", c.UnzipLimit, 512<<20)
	}
	if c.UnzipFiles != 10000 {
		t.Fatalf("UnzipFiles = %d, want 10000", c.UnzipFiles)
	}
	if FilesConfigDefaults(c) != c {
		t.Fatal("resolving twice changed the result")
	}
}

func TestAppFilesConfigDefaults(t *testing.T) {
	c := AppFilesConfig(context.Background())
	if c.ReadLimit != 50<<10 {
		t.Fatalf("ReadLimit = %d, want %d", c.ReadLimit, 50<<10)
	}
	if c.MaxBytes != 1<<30 {
		t.Fatalf("MaxBytes = %d, want %d", c.MaxBytes, 1<<30)
	}
}
