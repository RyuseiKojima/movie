package main

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadEnv(t *testing.T) {
    names := []string{"MOVIE_TEST_PLAIN", "MOVIE_TEST_DOUBLE", "MOVIE_TEST_SINGLE", "MOVIE_TEST_EXISTING"}
    for _, name := range names {
        previous, exists := os.LookupEnv(name)
        _ = os.Unsetenv(name)
        t.Cleanup(func() {
            if exists {
                _ = os.Setenv(name, previous)
            } else {
                _ = os.Unsetenv(name)
            }
        })
    }
    t.Setenv("MOVIE_TEST_EXISTING", "environment")
    file := filepath.Join(t.TempDir(), ".env")
    text := "# comment\nMOVIE_TEST_PLAIN=value # comment\nexport MOVIE_TEST_DOUBLE=\"quoted\"\nMOVIE_TEST_SINGLE='single'\nMOVIE_TEST_EXISTING=file\n"
    if err := os.WriteFile(file, []byte(text), 0600); err != nil {
        t.Fatal(err)
    }
    if err := loadEnv(file); err != nil {
        t.Fatal(err)
    }
    for name, expected := range map[string]string{"MOVIE_TEST_PLAIN": "value", "MOVIE_TEST_DOUBLE": "quoted", "MOVIE_TEST_SINGLE": "single", "MOVIE_TEST_EXISTING": "environment"} {
        if os.Getenv(name) != expected {
            t.Fatalf("%s mismatch", name)
        }
    }
    if err := loadEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
        t.Fatal(err)
    }
    _ = os.WriteFile(file, []byte("INVALID"), 0600)
    if err := loadEnv(file); err == nil {
        t.Fatal("expected malformed env error")
    }
}

func TestConfig(t *testing.T) {
    t.Setenv("PORT", "")
    t.Setenv("JEV_MODEL", "")
    cfg, err := readConfig("dist")
    if err != nil || cfg.Port != "3000" || cfg.JevModel != "jev-latest" {
        t.Fatalf("%+v %v", cfg, err)
    }
    for _, port := range []string{"abc", "0", "65536"} {
        t.Setenv("PORT", port)
        if _, err := readConfig("dist"); err == nil {
            t.Fatal("expected invalid port")
        }
    }
}
