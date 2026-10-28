package main

import "testing"

func TestResolvePort(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty uses default", raw: "", want: "8080"},
		{name: "valid port", raw: "3000", want: "3000"},
		{name: "not a number", raw: "abc", wantErr: true},
		{name: "zero rejected", raw: "0", wantErr: true},
		{name: "negative rejected", raw: "-1", wantErr: true},
		{name: "too large rejected", raw: "65536", wantErr: true},
		{name: "host prefix rejected", raw: "127.0.0.1:8080", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePort(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolvePort(%q): got %q, want error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePort(%q): unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("resolvePort(%q): got %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
