package script

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gr1m0h/k6-ec2/pkg/types"
)

const (
	// MaxPayloadSize is the maximum base64-encoded payload size in bytes.
	// SSM SendCommand parameters have a ~24KB limit; we reserve space for
	// the shell command wrapper around the payload.
	MaxPayloadSize = 20 * 1024

	// DefaultEntrypoint is the script path on the EC2 instance for single-file scripts.
	DefaultEntrypoint = "/tmp/test.js"

	// ArchiveBaseDir is the extraction directory for multi-file scripts on the EC2 instance.
	ArchiveBaseDir = "/tmp/k6-scripts"
)

// Payload is the resolved script ready for SSM SendCommand delivery.
type Payload struct {
	Content    []byte // gzip-compressed content (single file or tar archive)
	Entrypoint string // k6 run target path on the EC2 instance
	IsArchive  bool   // true if Content is a gzip-compressed tar archive
}

// Resolver resolves k6 test scripts from local sources into a Payload
// suitable for delivery via SSM SendCommand.
type Resolver struct{}

// NewResolver creates a new script Resolver.
func NewResolver() *Resolver {
	return &Resolver{}
}

// Resolve reads the script source specified in spec and produces a Payload
// for SSM delivery. The payload is gzip-compressed and checked against the
// SSM size limit.
func (r *Resolver) Resolve(spec *types.ScriptSpec) (*Payload, error) {
	var payload *Payload
	var err error

	switch {
	case spec.LocalFile != "":
		payload, err = r.resolveLocalFile(spec.LocalFile)
	case spec.LocalDir != "":
		payload, err = r.resolveLocalDir(spec.LocalDir, spec.Entrypoint)
	case spec.Inline != "":
		payload, err = r.resolveInline(spec.Inline)
	default:
		return nil, fmt.Errorf("no script source specified")
	}
	if err != nil {
		return nil, err
	}

	encodedSize := base64.StdEncoding.EncodedLen(len(payload.Content))
	if encodedSize > MaxPayloadSize {
		return nil, fmt.Errorf(
			"script payload too large for SSM delivery: %dKB encoded (max %dKB); reduce script size or split into smaller files",
			encodedSize/1024, MaxPayloadSize/1024,
		)
	}

	return payload, nil
}

func (r *Resolver) resolveLocalFile(path string) (*Payload, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file %s: %w", path, err)
	}

	compressed, err := gzipCompress(content)
	if err != nil {
		return nil, fmt.Errorf("failed to compress script: %w", err)
	}

	return &Payload{
		Content:    compressed,
		Entrypoint: DefaultEntrypoint,
	}, nil
}

func (r *Resolver) resolveLocalDir(dir, entrypoint string) (*Payload, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to access script directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	archived, err := tarGzDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to archive script directory: %w", err)
	}

	return &Payload{
		Content:    archived,
		Entrypoint: filepath.Join(ArchiveBaseDir, entrypoint),
		IsArchive:  true,
	}, nil
}

func (r *Resolver) resolveInline(content string) (*Payload, error) {
	compressed, err := gzipCompress([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("failed to compress inline script: %w", err)
	}

	return &Payload{
		Content:    compressed,
		Entrypoint: DefaultEntrypoint,
	}, nil
}

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func tarGzDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := tw.Write(content); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
