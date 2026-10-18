package tts

import "testing"

func TestLicenseAllowlist(t *testing.T) {
	cases := map[string]string{"license: CC-BY-4.0": "CC-BY-4.0", "License: Apache-2.0": "Apache-2.0", "license: MIT": "MIT"}
	for input, want := range cases {
		got, _ := licenseFor(input)
		if got != want {
			t.Errorf("licenseFor(%q)=%q, want %q", input, got, want)
		}
	}
	for _, input := range []string{"Research only", "license: MITIGATE", "Blizzard license"} {
		if got, _ := licenseFor(input); got != "" {
			t.Errorf("unrecognized/restricted license %q was allowed as %s", input, got)
		}
	}
}
