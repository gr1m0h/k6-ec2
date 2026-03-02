package script

import "testing"

func TestParseS3URI(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantBucket string
		wantKey    string
		wantErr    bool
	}{
		{
			"valid simple",
			"s3://my-bucket/test.js",
			"my-bucket", "test.js",
			false,
		},
		{
			"valid with prefix",
			"s3://my-bucket/prefix/subdir/test.js",
			"my-bucket", "prefix/subdir/test.js",
			false,
		},
		{
			"missing s3 prefix",
			"https://my-bucket/test.js",
			"", "",
			true,
		},
		{
			"no key",
			"s3://my-bucket",
			"", "",
			true,
		},
		{
			"no key trailing slash",
			"s3://my-bucket/",
			"", "",
			true,
		},
		{
			"empty bucket",
			"s3:///key",
			"", "",
			true,
		},
		{
			"empty string",
			"",
			"", "",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := parseS3URI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseS3URI(%q) error = %v, wantErr %v", tt.uri, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if loc.Bucket != tt.wantBucket {
				t.Errorf("bucket: expected %q, got %q", tt.wantBucket, loc.Bucket)
			}
			if loc.Key != tt.wantKey {
				t.Errorf("key: expected %q, got %q", tt.wantKey, loc.Key)
			}
		})
	}
}
