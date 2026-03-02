package main

import (
	"testing"
)

func TestParseS3URI(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantBucket string
		wantKey    string
		wantErr    bool
	}{
		{
			name:       "valid simple",
			uri:        "s3://my-bucket/my-key.js",
			wantBucket: "my-bucket",
			wantKey:    "my-key.js",
		},
		{
			name:       "valid with nested key",
			uri:        "s3://my-bucket/path/to/script.js",
			wantBucket: "my-bucket",
			wantKey:    "path/to/script.js",
		},
		{
			name:       "valid with deep path",
			uri:        "s3://bucket-name/a/b/c/d/test.tar.gz",
			wantBucket: "bucket-name",
			wantKey:    "a/b/c/d/test.tar.gz",
		},
		{
			name:    "missing s3 prefix",
			uri:     "https://my-bucket/my-key.js",
			wantErr: true,
		},
		{
			name:    "empty string",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "s3 prefix only",
			uri:     "s3://",
			wantErr: true,
		},
		{
			name:    "bucket only no key",
			uri:     "s3://my-bucket",
			wantErr: true,
		},
		{
			name:    "bucket with trailing slash",
			uri:     "s3://my-bucket/",
			wantErr: true,
		},
		{
			name:    "empty bucket",
			uri:     "s3:///key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := parseS3URI(tt.uri)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseS3URI(%q) should return error", tt.uri)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseS3URI(%q) unexpected error: %v", tt.uri, err)
			}
			if loc.Bucket != tt.wantBucket {
				t.Errorf("Bucket = %q, want %q", loc.Bucket, tt.wantBucket)
			}
			if loc.Key != tt.wantKey {
				t.Errorf("Key = %q, want %q", loc.Key, tt.wantKey)
			}
		})
	}
}
