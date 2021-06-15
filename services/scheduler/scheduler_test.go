package main

import "testing"

func Test_generateRSAKeyPair(t *testing.T) {
	var tests []struct {
		name  string
		want  string
		want1 string
	}
	tests = append(tests, struct {
		name  string
		want  string
		want1 string
	}{name: "Pokus", want: "Something", want1: "Something else"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := generateRSAKeyPair()
			t.Log("Private key", got)
			t.Log("Public key", got1)
			if got != tt.want {
				t.Errorf("generateRSAKeyPair() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("generateRSAKeyPair() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
