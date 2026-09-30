package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReturnPathCannotEscapeFrontend(t *testing.T) {
	for _, path := range []string{"https://evil.test/", "//evil.test/", "/%2fevil.test/", "/%5cevil.test/", "/\\evil.test/", "/%0aevil", "/#fragment", "/%zz"} {
		t.Run(path, func(t *testing.T) {
			if _, err := ResolveReturnPath("https://frontend.test", path); err == nil {
				t.Fatalf("accepted unsafe return path %q", path)
			}
		})
	}

	got, err := ResolveReturnPath("https://frontend.test", "/bills?tab=history")
	if err != nil || got != "https://frontend.test/bills?tab=history" {
		t.Fatalf("valid return path: %q, %v", got, err)
	}
}

func TestRegistryRejectsUnknownFieldsSecretsAndDuplicateClients(t *testing.T) {
	t.Setenv("INTANIA_GAMES_CLIENT_SECRET", strings.Repeat("s", 32))
	contents, err := os.ReadFile("../../config/auth.development.yaml")
	if err != nil {
		t.Fatal(err)
	}

	load := func(contents string) error {
		t.Helper()
		path := filepath.Join(t.TempDir(), "auth.yaml")
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadAuthRegistry(path, "development", "http://localhost:3001", os.Getenv)
		return err
	}

	if err := load(string(contents)); err != nil {
		t.Fatal(err)
	}

	for name, contents := range map[string]string{
		"unknown field":        string(contents) + "\nunknown: true\n",
		"duplicate ID":         strings.ReplaceAll(string(contents), "id: intania-games", "id: intania-888-web"),
		"HTTP remote callback": strings.ReplaceAll(string(contents), "http://localhost:3002/auth/callback", "http://remote.test/callback"),
		"wildcard callback":    strings.ReplaceAll(string(contents), "http://localhost:3002/auth/callback", "https://*.example.test/callback"),
		"invalid lifetime":     strings.ReplaceAll(string(contents), "authorization_code_seconds: 60", "authorization_code_seconds: 0"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := load(contents); err == nil {
				t.Fatal("accepted invalid registry")
			}
		})
	}

	t.Setenv("INTANIA_GAMES_CLIENT_SECRET", "")
	if err := load(string(contents)); err == nil {
		t.Fatal("accepted missing client secret")
	}
}
