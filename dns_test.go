package main

import (
	"testing"
)

func TestTypeToStr(t *testing.T) {
	tests := []struct {
		in        uint16
		wantStr   string
		wantFound bool
	}{
		{1, "A", true},
		{28, "AAAA", true},
		{5, "CNAME", true},
		{16, "TXT", true},
		{255, "", false},
	}

	for _, tt := range tests {
		gotStr, gotFound := typeToStr(tt.in)

		if gotStr != tt.wantStr {
			t.Errorf("Str of typeToStr(%d) = %s, want %s", tt.in, gotStr, tt.wantStr)
		}

		if gotFound != tt.wantFound {
			t.Errorf("Found of typeToStr(%d) = %t, want %t", tt.in, gotFound, tt.wantFound)
		}
	}
}

func TestStrToType(t *testing.T) {
	tests := []struct {
		in        string
		wantType  uint16
		wantFound bool
	}{
		{"A", 1, true},
		{"AAAA", 28, true},
		{"CNAME", 5, true},
		{"TXT", 16, true},
		{"ABC", 0, false},
	}

	for _, tt := range tests {
		gotType, gotFound := strToType(tt.in)

		if gotType != tt.wantType {
			t.Errorf("Str of strToType(%s) = %d, want %d", tt.in, gotType, tt.wantType)
		}

		if gotFound != tt.wantFound {
			t.Errorf("Found of strToType(%s) = %t, want %t", tt.in, gotFound, tt.wantFound)
		}
	}
}
