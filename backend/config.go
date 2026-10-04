package main

import (
    "bufio"
    "errors"
    "fmt"
    "os"
    "strconv"
    "strings"
)

type config struct {
    TMDBToken string
    JevKey    string
    JevModel  string
    Port      string
    StaticDir string
}

// Existing environment variables take precedence over the optional .env file.
func loadEnv(path string) error {
    file, err := os.Open(path)
    if errors.Is(err, os.ErrNotExist) {
        return nil
    }
    if err != nil {
        return err
    }
    defer file.Close()
    scanner := bufio.NewScanner(file)
    for line := 1; scanner.Scan(); line++ {
        text := strings.TrimSpace(scanner.Text())
        if text == "" || strings.HasPrefix(text, "#") {
            continue
        }
        text = strings.TrimPrefix(text, "export ")
        key, value, ok := strings.Cut(text, "=")
        key, value = strings.TrimSpace(key), strings.TrimSpace(value)
        if !ok || key == "" {
            return fmt.Errorf(".env:%d: invalid assignment", line)
        }
        if strings.HasPrefix(value, "\"") {
            value, err = strconv.Unquote(value)
            if err != nil {
                return fmt.Errorf(".env:%d: invalid quoted value", line)
            }
        } else if strings.HasPrefix(value, "'") {
            if len(value) < 2 || !strings.HasSuffix(value, "'") {
                return fmt.Errorf(".env:%d: invalid quoted value", line)
            }
            value = value[1 : len(value)-1]
        } else if index := strings.Index(value, " #"); index >= 0 {
            value = strings.TrimSpace(value[:index])
        }
        if _, exists := os.LookupEnv(key); !exists {
            if err := os.Setenv(key, value); err != nil {
                return fmt.Errorf(".env:%d: invalid variable name", line)
            }
        }
    }
    return scanner.Err()
}

func readConfig(staticDir string) (config, error) {
    cfg := config{
        TMDBToken: strings.TrimSpace(os.Getenv("TMDB_ACCESS_TOKEN")),
        JevKey:    strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")),
        JevModel:  strings.TrimSpace(os.Getenv("JEV_MODEL")),
        Port:      strings.TrimSpace(os.Getenv("PORT")),
        StaticDir: staticDir,
    }
    if cfg.JevModel == "" {
        cfg.JevModel = "jev-latest"
    }
    if cfg.Port == "" {
        cfg.Port = "3000"
    }
    port, err := strconv.Atoi(cfg.Port)
    if err != nil || port < 1 || port > 65535 {
        return cfg, errors.New("PORT must be between 1 and 65535")
    }
    return cfg, nil
}
