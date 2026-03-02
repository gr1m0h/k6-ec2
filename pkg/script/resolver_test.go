package script

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gr1m0h/k6-ec2/pkg/types"
)

func TestResolve_LocalFile(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "test.js")
	content := "import http from 'k6/http'; export default function() { http.get('https://test.k6.io'); }"
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver()
	payload, err := r.Resolve(&types.ScriptSpec{LocalFile: scriptPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.Entrypoint != DefaultEntrypoint {
		t.Errorf("expected entrypoint %q, got %q", DefaultEntrypoint, payload.Entrypoint)
	}
	if payload.IsArchive {
		t.Error("expected IsArchive=false for single file")
	}

	decompressed, err := gzipDecompress(payload.Content)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	if string(decompressed) != content {
		t.Errorf("content mismatch: expected %q, got %q", content, string(decompressed))
	}
}

func TestResolve_LocalDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte("import { helper } from './helper.js';"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helper.js"), []byte("export function helper() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver()
	payload, err := r.Resolve(&types.ScriptSpec{LocalDir: dir, Entrypoint: "main.js"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedEntrypoint := filepath.Join(ArchiveBaseDir, "main.js")
	if payload.Entrypoint != expectedEntrypoint {
		t.Errorf("expected entrypoint %q, got %q", expectedEntrypoint, payload.Entrypoint)
	}
	if !payload.IsArchive {
		t.Error("expected IsArchive=true for directory")
	}

	files := extractTarGz(t, payload.Content)
	if _, ok := files["main.js"]; !ok {
		t.Error("expected main.js in archive")
	}
	if _, ok := files["helper.js"]; !ok {
		t.Error("expected helper.js in archive")
	}
}

func TestResolve_LocalDir_WithSubdirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "lib")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte("import { util } from './lib/util.js';"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "util.js"), []byte("export function util() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver()
	payload, err := r.Resolve(&types.ScriptSpec{LocalDir: dir, Entrypoint: "main.js"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := extractTarGz(t, payload.Content)
	if _, ok := files["main.js"]; !ok {
		t.Error("expected main.js in archive")
	}
	if _, ok := files[filepath.Join("lib", "util.js")]; !ok {
		t.Error("expected lib/util.js in archive")
	}
}

func TestResolve_Inline(t *testing.T) {
	content := "export default function() { console.log('hello'); }"

	r := NewResolver()
	payload, err := r.Resolve(&types.ScriptSpec{Inline: content})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.Entrypoint != DefaultEntrypoint {
		t.Errorf("expected entrypoint %q, got %q", DefaultEntrypoint, payload.Entrypoint)
	}
	if payload.IsArchive {
		t.Error("expected IsArchive=false for inline")
	}

	decompressed, err := gzipDecompress(payload.Content)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	if string(decompressed) != content {
		t.Errorf("content mismatch: expected %q, got %q", content, string(decompressed))
	}
}

func TestResolve_NoSource(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(&types.ScriptSpec{})
	if err == nil {
		t.Fatal("expected error for empty spec")
	}
}

func TestResolve_FileNotFound(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(&types.ScriptSpec{LocalFile: "/nonexistent/test.js"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestResolve_DirNotFound(t *testing.T) {
	r := NewResolver()
	_, err := r.Resolve(&types.ScriptSpec{LocalDir: "/nonexistent/dir", Entrypoint: "main.js"})
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestResolve_NotADirectory(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver()
	_, err := r.Resolve(&types.ScriptSpec{LocalDir: f, Entrypoint: "main.js"})
	if err == nil {
		t.Fatal("expected error for file passed as localDir")
	}
}

func TestResolve_PayloadTooLarge(t *testing.T) {
	dir := t.TempDir()
	// Random data is incompressible by gzip, so 100KB of random bytes
	// will remain ~100KB after gzip, far exceeding the 20KB limit.
	rng := rand.New(rand.NewPCG(42, 0))
	buf := make([]byte, 100*1024)
	for i := range buf {
		buf[i] = byte(rng.IntN(256))
	}
	if err := os.WriteFile(filepath.Join(dir, "test.js"), buf, 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver()
	_, err := r.Resolve(&types.ScriptSpec{LocalFile: filepath.Join(dir, "test.js")})
	if err == nil {
		t.Fatal("expected error for oversized payload")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' in error, got: %v", err)
	}
}

func TestResolve_GzipCompression(t *testing.T) {
	content := strings.Repeat("import http from 'k6/http';\n", 100)

	r := NewResolver()
	payload, err := r.Resolve(&types.ScriptSpec{Inline: content})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	encodedSize := base64.StdEncoding.EncodedLen(len(payload.Content))
	rawSize := len(content)
	if encodedSize >= rawSize {
		t.Errorf("expected compression to reduce size: raw=%d, encoded=%d", rawSize, encodedSize)
	}
}

// helpers

func gzipDecompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func extractTarGz(t *testing.T, data []byte) map[string]string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	files := make(map[string]string)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}
		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("failed to read file content: %v", err)
			}
			files[header.Name] = string(content)
		}
	}
	return files
}
