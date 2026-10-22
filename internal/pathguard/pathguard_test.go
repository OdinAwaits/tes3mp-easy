package pathguard

import (
	"errors"
	"testing"
)

func TestBase(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "mod.esp"},
		{name: "mods-2026-10-22_12-00-00.tar.gz"},
		{name: "test.lua"},
		{name: "_mod-1"},
		{name: "", wantErr: true},
		{name: ".", wantErr: true},
		{name: "..", wantErr: true},
		{name: "plugins/mod.esp", wantErr: true},
		{name: `plugins\mod.esp`, wantErr: true},
		{name: "C:mod.esp", wantErr: true},
		{name: "mod\x00.esp", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Base(tt.name)
			if tt.wantErr {
				if !errors.Is(err, ErrUnsafePath) {
					t.Fatalf("Base(%q) = %v, want ErrUnsafePath", tt.name, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Base(%q) = %v", tt.name, err)
			}
		})
	}
}

func TestPath(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: "plugins/mod.esp", want: "plugins/mod.esp"},
		{name: "plugins/", want: "plugins"},
		{name: "./plugins/mod.esp", want: "plugins/mod.esp"},
		{name: "plugins//mod.esp", want: "plugins/mod.esp"},
		{name: "plugins/./mod.esp", want: "plugins/mod.esp"},
		{name: "scripts/test.lua", want: "scripts/test.lua"},
		{name: "", wantErr: true},
		{name: ".", wantErr: true},
		{name: "..", wantErr: true},
		{name: "../evil", wantErr: true},
		{name: "plugins/../../evil", wantErr: true},
		{name: "plugins/..", wantErr: true},
		{name: "/etc/passwd", wantErr: true},
		{name: "//server/share", wantErr: true},
		{name: `plugins\mod.esp`, wantErr: true},
		{name: `\evil`, wantErr: true},
		{name: "C:/evil", wantErr: true},
		{name: "C:evil", wantErr: true},
		{name: "plugins/\x00evil", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Path(tt.name)
			if tt.wantErr {
				if !errors.Is(err, ErrUnsafePath) {
					t.Fatalf("Path(%q) = %q, %v; want ErrUnsafePath", tt.name, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Path(%q) = %v", tt.name, err)
			}
			if got != tt.want {
				t.Fatalf("Path(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
