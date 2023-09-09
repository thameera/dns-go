package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"testing"
)

// If we run `go test -update`, it will update the golden
// files. Should be run when adding a new test or only if
// an update is really necessary.
var update = flag.Bool("update", false, "update the golden files")

// Function to read/update "golden values" which are known
// good outputs.
func goldenValue(t *testing.T, goldenFile string, got []byte) []byte {
	t.Helper()
	goldenPath := "testdata/" + goldenFile + ".golden"

	if *update {
		ioutil.WriteFile(goldenPath, got, 0644)

		return got
	}

	want, err := ioutil.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed reading .golden file: %s", err)
	}

	return want
}

func compareByteArrays(t *testing.T, testName string, got, want []byte) {
	if !bytes.Equal(got, want) {
		t.Errorf("Output doesn't match golden file in test '%s'.\nWant:\n%v\nGot:\n%v\n", testName, want, got)
	}
}

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

func TestCreateHeader(t *testing.T) {
	got, err := createHeader()
	if err != nil {
		t.Fatalf("Expected nil error, but got: %s", err)
	}

	want := goldenValue(t, "createHeader", got[2:])
	// We used got[2:] there because the first two bytes are a random ID

	compareByteArrays(t, "Test header", got[2:], want)
}
