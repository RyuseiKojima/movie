package main

import (
    "encoding/json"
    "errors"
    "io"
    "io/fs"
    "net/http"
    "net/url"
    "os"
    "path"
    "strings"
)

type app struct {
    cfg        config
    client     *http.Client
    staticRoot *os.Root
}

func newApp(cfg config, client *http.Client) (*app, error) {
    root, err := os.OpenRoot(cfg.StaticDir)
    if err != nil {
        return nil, err
    }
    return &app{cfg: cfg, client: client, staticRoot: root}, nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.Header().Set("Cache-Control", "no-store")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, map[string]string{"error": message})
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
    if r.Method == method {
        return true
    }
    w.Header().Set("Allow", method)
    writeError(w, http.StatusMethodNotAllowed, "このメソッドは使用できません。")
    return false
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/api/config":
        if requireMethod(w, r, http.MethodGet) {
            writeJSON(w, 200, map[string]bool{"demo": a.cfg.TMDBToken == "", "jev": a.cfg.JevKey != ""})
        }
    case "/api/search":
        if !requireMethod(w, r, http.MethodGet) {
            return
        }
        query := strings.TrimSpace(r.URL.Query().Get("q"))
        if inputLength(query) > 200 {
            writeError(w, 400, "検索語は200文字以内で入力してください。")
            return
        }
        if a.cfg.TMDBToken == "" {
            results := make([]movie, 0)
            for _, candidate := range demoMovies {
                if query == "" || strings.Contains(candidate.Title, query) {
                    results = append(results, candidate)
                }
            }
            writeJSON(w, 200, map[string]any{"results": results, "demo": true})
            return
        }
        endpoint := "movie/popular"
        params := url.Values{}
        if query != "" {
            endpoint = "search/movie"
            params.Set("query", query)
        }
        var data json.RawMessage
        if err := a.tmdb(r.Context(), endpoint, params, &data); err != nil {
            writeError(w, 400, err.Error())
            return
        }
        writeJSON(w, 200, data)
    case "/api/recommend":
        if !requireMethod(w, r, http.MethodPost) {
            return
        }
        body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 100000))
        if err != nil {
            var tooLarge *http.MaxBytesError
            if errors.As(err, &tooLarge) {
                writeError(w, 413, "登録データが大きすぎます。")
            } else {
                writeError(w, 400, "入力形式が不正です。")
            }
            return
        }
        if !json.Valid(body) {
            writeError(w, 400, "入力形式が不正です。")
            return
        }
        var input recommendationInput
        if err := json.Unmarshal(body, &input); err != nil {
            writeError(w, 400, invalidInput)
            return
        }
        result, err := a.recommend(r.Context(), input)
        if err != nil {
            writeError(w, 400, err.Error())
            return
        }
        writeJSON(w, 200, result)
    default:
        a.serveStatic(w, r)
    }
}

func (a *app) serveStatic(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet && r.Method != http.MethodHead {
        w.Header().Set("Allow", "GET, HEAD")
        writeError(w, 405, "このメソッドは使用できません。")
        return
    }
    name := strings.TrimPrefix(r.URL.Path, "/")
    if name == "" {
        name = "index.html"
    }
    types := map[string]string{".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml"}
    contentType, ok := types[path.Ext(name)]
    if !ok || !fs.ValidPath(name) {
        writeError(w, 404, "見つかりません。")
        return
    }
    file, err := a.staticRoot.Open(name)
    if err != nil {
        writeError(w, 404, "見つかりません。")
        return
    }
    defer file.Close()
    info, err := file.Stat()
    if err != nil || !info.Mode().IsRegular() {
        writeError(w, 404, "見つかりません。")
        return
    }
    w.Header().Set("Content-Type", contentType+"; charset=utf-8")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    http.ServeContent(w, r, name, info.ModTime(), file)
}
