package main

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "io/fs"
    "net/http"
    "net/url"
    "os"
    "path"
    "strings"
)

// The library (up to 500 movies) and up to 100 candidates with overviews are sent together.
const maxRecommendBytes = 1_000_000

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
        yearFrom, yearTo, err := parseYearRange(r.URL.Query())
        if err != nil {
            writeError(w, 400, err.Error())
            return
        }
        inRange := func(candidate movie) bool { return inYearRange(candidate, yearFrom, yearTo) }
        if a.cfg.TMDBToken == "" {
            results := make([]movie, 0)
            for _, candidate := range demoMovies {
                if (query == "" || strings.Contains(candidate.Title, query)) && inRange(candidate) {
                    results = append(results, candidate)
                }
            }
            writeJSON(w, 200, map[string]any{"results": results, "demo": true})
            return
        }
        if query == "" {
            // A random sample of popular movies, narrowed by release date when years are given.
            path, params := "movie/popular", url.Values{}
            if yearFrom != nil || yearTo != nil {
                path = "discover/movie"
                params.Set("sort_by", "popularity.desc")
                if yearFrom != nil {
                    params.Set("primary_release_date.gte", fmt.Sprintf("%04d-01-01", *yearFrom))
                }
                if yearTo != nil {
                    params.Set("primary_release_date.lte", fmt.Sprintf("%04d-12-31", *yearTo))
                }
            }
            results, err := a.fetchMovies(r.Context(), path, params, func(movie) bool { return true })
            if err != nil {
                writeError(w, 400, err.Error())
                return
            }
            writeJSON(w, 200, map[string]any{"results": results})
            return
        }
        params := url.Values{}
        params.Set("query", query)
        var data struct {
            Results []movie `json:"results"`
        }
        if err := a.tmdb(r.Context(), "search/movie", params, &data); err != nil {
            writeError(w, 400, err.Error())
            return
        }
        // TMDB search accepts only a single year, so filter ranges locally.
        results := make([]movie, 0, len(data.Results))
        for _, candidate := range data.Results {
            if inRange(candidate) {
                results = append(results, candidate)
            }
        }
        writeJSON(w, 200, map[string]any{"results": results})
    case "/api/recommend":
        if !requireMethod(w, r, http.MethodPost) {
            return
        }
        body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRecommendBytes))
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
