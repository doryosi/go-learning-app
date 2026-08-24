package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("PORT", "9090")
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != ":9090" {
		t.Fatalf("Address = %q, want :9090", got.Address)
	}
}
func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("PORT", "nope")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
