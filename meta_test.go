package main

import "testing"

func TestMetaDataKey(t *testing.T) {

	tests := []struct {
		in      MetaDataKey
		valid   bool
		name    string
		version string
	}{
		{
			in:      "fp.topics.0.18",
			valid:   true,
			name:    "topics",
			version: "0.18",
		},
		{
			in:    "fp.topics.0",
			valid: false,
		},
		{
			in:    "topics.0.18",
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(string(test.in), func(t *testing.T) {
			valid := test.in.Valid()
			if valid != test.valid {
				t.Errorf("expected valid %T, got %T", test.valid, valid)
				return
			}

			if valid {
				name := test.in.Name()
				if name != test.name {
					t.Errorf("expected name %s, got %s", test.name, name)
				}

				version := test.in.Version()
				if version != test.version {
					t.Errorf("expected version %s, got %s", test.version, version)
				}
			}
		})
	}
}
