package config

import "testing"

func TestLoad_Success(t *testing.T) {
	t.Setenv(dbURLEnvVar, "postgres://gator:gator@localhost:5432/gator?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := "postgres://gator:gator@localhost:5432/gator?sslmode=disable"
	if cfg.DBURL != want {
		t.Errorf("DBURL = %q, want %q", cfg.DBURL, want)
	}
}

func TestLoad_MissingDBURL(t *testing.T) {
	t.Setenv(dbURLEnvVar, "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when GATOR_DB_URL is unset, got nil")
	}
}
