package types

import "testing"

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "my-load-test", false},
		{"empty name", "", true},
		{"single char", "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateScript(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ScriptSpec
		wantErr bool
	}{
		{
			"localFile only",
			&ScriptSpec{LocalFile: "./test.js"},
			false,
		},
		{
			"localDir with entrypoint",
			&ScriptSpec{LocalDir: "./scripts", Entrypoint: "main.js"},
			false,
		},
		{
			"inline only",
			&ScriptSpec{Inline: "export default function() {}"},
			false,
		},
		{
			"no source",
			&ScriptSpec{},
			true,
		},
		{
			"two sources localFile and localDir",
			&ScriptSpec{LocalFile: "./test.js", LocalDir: "./scripts", Entrypoint: "main.js"},
			true,
		},
		{
			"two sources localDir and inline",
			&ScriptSpec{LocalDir: "./scripts", Entrypoint: "main.js", Inline: "test"},
			true,
		},
		{
			"localDir without entrypoint",
			&ScriptSpec{LocalDir: "./scripts"},
			true,
		},
		{
			"entrypoint without localDir",
			&ScriptSpec{LocalFile: "./test.js", Entrypoint: "main.js"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScript(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateScript() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateParallelism(t *testing.T) {
	tests := []struct {
		name    string
		input   int32
		wantErr bool
	}{
		{"valid min", 1, false},
		{"valid max", 100, false},
		{"valid mid", 50, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"over max", 101, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParallelism(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParallelism(%d) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTimeout(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"valid minutes", "30m", false},
		{"valid hours", "2h", false},
		{"valid seconds", "90s", false},
		{"valid composite", "1h30m", false},
		{"invalid format", "thirty", true},
		{"invalid unit", "30x", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeout(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTimeout(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCleanup(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty default", "", false},
		{"always", "always", false},
		{"on-success", "on-success", false},
		{"never", "never", false},
		{"invalid policy", "sometimes", true},
		{"typo", "allways", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCleanup(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCleanup(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
