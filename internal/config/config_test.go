package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveIsPrivateAndLoadRoundTrips(t *testing.T) {
	t.Setenv("EOS_PRINT_AGENT_HOME", t.TempDir())
	if _, err := Load(); err != ErrNotPaired {
		t.Fatalf("sin vincular: %v", err)
	}
	if err := Save(&Config{URL: "https://demo.example.com", Token: "secreto-1234"}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(Dir(), "config.json"))
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos %v", info.Mode().Perm())
	}
	c, err := Load()
	if err != nil || c.Token != "secreto-1234" {
		t.Fatal(err)
	}
	if Hint(c.Token) != "…1234" {
		t.Fatal(Hint(c.Token))
	}
}

func TestNormalizeURLRequiresHTTPSExceptLocalhost(t *testing.T) {
	cases := map[string]string{
		"demo.example.com/login":                "https://demo.example.com",
		"http://starkindustries.localhost:8000": "http://starkindustries.localhost:8000",
	}
	for in, want := range cases {
		if got, err := NormalizeURL(in); err != nil || got != want {
			t.Fatalf("%s → %s (%v)", in, got, err)
		}
	}
	if _, err := NormalizeURL("http://demo.example.com"); err == nil {
		t.Fatal("http sin TLS fuera de localhost debe rechazarse")
	}
}
