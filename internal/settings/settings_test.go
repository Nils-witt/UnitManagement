package settings

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	for _, u := range []string{
		"",
		"https://sgx.geodatenzentrum.de/gdz_basemapde_vektor/styles/bm_web_col.json",
		"http://localhost:8081/style.json",
	} {
		if err := (Settings{MapStyleURL: u}).Validate(); err != nil {
			t.Errorf("%q: %v", u, err)
		}
	}
	for _, u := range []string{
		"style.json",
		"/styles/style.json",
		"ftp://example.com/style.json",
		"javascript:alert(1)",
		"https://user:pass@example.com/style.json",
		"https://",
		"https://a.example;script-src/style.json",
	} {
		if err := (Settings{MapStyleURL: u}).Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: err = %v, want ErrInvalid", u, err)
		}
	}
}

func TestMapOrigin(t *testing.T) {
	for in, want := range map[string]string{
		"": "",
		"https://tiles.example.com/styles/a.json?key=1": "https://tiles.example.com",
		"http://localhost:8081/style.json":              "http://localhost:8081",
	} {
		if got := (Settings{MapStyleURL: in}).MapOrigin(); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
