package manifest

import "testing"

func TestValidateVersionUpgrade(t *testing.T) {
	tests := []struct {
		current, desired string
		wantErr          bool
	}{
		{"1.35.0", "1.35.0", false},
		{"1.35.0", "1.35.2", false},
		{"1.35.2", "1.36.0", false},
		{"v1.35.0", "1.36.1", false},
		{"1.35.2", "1.35.1", true},
		{"1.36.0", "1.35.0", true},
		{"1.34.0", "1.36.0", true},
		{"1.35.0", "2.0.0", true},
		{"1.35.0", "not-a-version", true},
		{"", "1.35.0", true},
	}

	for _, tt := range tests {
		err := ValidateVersionUpgrade(tt.current, tt.desired)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateVersionUpgrade(%q, %q) error = %v, wantErr %v", tt.current, tt.desired, err, tt.wantErr)
		}
	}
}
