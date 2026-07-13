package server

import (
	"testing"
)

func TestParseTargetValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    float64
		wantErr bool
	}{
		{name: "default", value: "", want: 80},
		{name: "explicit", value: "12.5", want: 12.5},
		{name: "malformed", value: "not-a-number", wantErr: true},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "nan", value: "NaN", wantErr: true},
		{name: "infinity", value: "+Inf", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTargetValue(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTargetValue(%q) error = %v, wantErr %t", tt.value, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("parseTargetValue(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
