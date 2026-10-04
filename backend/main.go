package main

import (
    "context"
    "errors"
    "flag"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    envFile := flag.String("env", ".env", "environment file")
    staticDir := flag.String("static", "dist", "frontend directory")
    flag.Parse()
    if err := loadEnv(*envFile); err != nil {
        log.Fatal(err)
    }
    cfg, err := readConfig(*staticDir)
    if err != nil {
        log.Fatal(err)
    }
    app, err := newApp(cfg, &http.Client{Timeout: 15 * time.Second})
    if err != nil {
        log.Fatal(err)
    }
    defer app.staticRoot.Close()
    srv := &http.Server{
        Addr:              "127.0.0.1:" + cfg.Port,
        Handler:           app,
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       10 * time.Second,
        WriteTimeout:      40 * time.Second,
        IdleTimeout:       60 * time.Second,
    }
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    shutdownDone := make(chan struct{})
    go func() {
        defer close(shutdownDone)
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := srv.Shutdown(shutdownCtx); err != nil {
            log.Print(err)
        }
    }()
    log.Printf("Movie Shelf: http://localhost:%s", cfg.Port)
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Fatal(err)
    }
    <-shutdownDone
}
