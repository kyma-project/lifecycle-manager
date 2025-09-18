package metrics

import (
	"testing"
)

func TestParseGodebugFips140only(t *testing.T) {
	t.Parallel()

	tests := []struct {
		godebug string
		want    string
	}{
		{"fips140=only", "only"},
		{"fips140=on", "on"},
		{"fips140=debug", "debug"},
		{"fips140=off", "off"},
		{
			"fips140=someOtherValue",
			"off",
		}, // Such a value is refused by standard library at runtime, but here let's just default to "off"
		{"fips140=debug, fips140=only", "only"},
		{"fips140=off,fips140=on,fips140=only", "only"},  // The last one wins
		{"fips140=only, fips140=on, fips140=off", "off"}, // Spaces around values should not affect the result
		{"fips140=only , fips140=off ,fips140=on", "on"}, // Spaces around values should not affect the result
	}

	for _, tt := range tests {
		t.Run(tt.godebug, func(t *testing.T) {
			t.Parallel()
			if got := parseGodebugFipsMode(tt.godebug); got != tt.want {
				t.Errorf("parseGodebugFips140only(%q) = %v, want %v", tt.godebug, got, tt.want)
			}
		})
	}
}
