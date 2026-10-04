package main

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(body string, status int) *http.Response {
    return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func testApp(t *testing.T, cfg config, transport transportFunc) *app {
    t.Helper()
    dir := t.TempDir()
    if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>Movie Shelf</title>"), 0600); err != nil {
        t.Fatal(err)
    }
    cfg.StaticDir = dir
    if cfg.JevModel == "" {
        cfg.JevModel = "jev-latest"
    }
    client := &http.Client{Transport: transport}
    a, err := newApp(cfg, client)
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = a.staticRoot.Close() })
    return a
}

func call(a *app, method, endpoint, body string) *httptest.ResponseRecorder {
    w := httptest.NewRecorder()
    a.ServeHTTP(w, httptest.NewRequest(method, endpoint, strings.NewReader(body)))
    return w
}

func TestRoutesAndValidation(t *testing.T) {
    a := testApp(t, config{}, nil)
    cases := []struct {
        method, path, body string
        status             int
        contains           string
    }{
        {"GET", "/api/config", "", 200, `"demo":true`},
        {"GET", "/api/search?q=喫茶店", "", 200, "雨の日の喫茶店"},
        {"GET", "/api/search?q=不存在", "", 200, `"results":[]`},
        {"GET", "/api/search?q=" + strings.Repeat("あ", 201), "", 400, "200文字"},
        {"GET", "/", "", 200, "Movie Shelf"},
        {"GET", "/unknown", "", 404, "見つかりません"},
        {"GET", "/../index.html", "", 404, "見つかりません"},
        {"POST", "/api/recommend", "{", 400, "入力形式"},
        {"POST", "/api/recommend", `{} {}`, 400, "入力形式"},
        {"POST", "/api/recommend", strings.Repeat("x", 100001), 413, "大きすぎ"},
        {"POST", "/api/recommend", `{"mood":"x","library":null}`, 400, "形式が不正"},
        {"POST", "/api/recommend", `{"mood":123,"library":[]}`, 400, "形式が不正"},
        {"POST", "/api/recommend", `{"mood":"   ","library":[]}`, 400, "今の気分"},
        {"POST", "/api/recommend", `{"mood":"x","library":[]}`, 400, "APIキーが必要"},
        {"GET", "/api/recommend", "", 405, "メソッド"},
        {"POST", "/api/search", "", 405, "メソッド"},
    }
    for _, tc := range cases {
        t.Run(tc.method+tc.path+tc.contains, func(t *testing.T) {
            w := call(a, tc.method, tc.path, tc.body)
            if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
                t.Fatalf("got %d %s", w.Code, w.Body.String())
            }
        })
    }
    outside := filepath.Join(t.TempDir(), "secret.js")
    _ = os.WriteFile(outside, []byte("secret"), 0600)
    if err := os.Symlink(outside, filepath.Join(a.cfg.StaticDir, "escape.js")); err != nil {
        t.Fatal(err)
    }
    if w := call(a, "GET", "/escape.js", ""); w.Code != 404 {
        t.Fatal("symlink escaped static root")
    }
}

func TestTMDBAuthentication(t *testing.T) {
    for _, token := range []string{strings.Repeat("a", 32), "read.access.token"} {
        t.Run(token, func(t *testing.T) {
            count := 0
            a := testApp(t, config{TMDBToken: token}, func(r *http.Request) (*http.Response, error) {
                count++
                if r.URL.Host != "api.themoviedb.org" || r.URL.Path != "/3/search/movie" || r.URL.Query().Get("query") != "日本" {
                    t.Fatalf("unexpected URL %s", r.URL)
                }
                if r.URL.Query().Get("language") != "ja-JP" || r.URL.Query().Get("include_adult") != "false" {
                    t.Fatal("missing defaults")
                }
                if len(token) == 32 {
                    if r.URL.Query().Get("api_key") != token || r.Header.Get("Authorization") != "" {
                        t.Fatal("API key auth")
                    }
                } else if r.Header.Get("Authorization") != "Bearer "+token || r.URL.Query().Has("api_key") {
                    t.Fatal("bearer auth")
                }
                return jsonResponse(`{"results":[],"page":1}`, 200), nil
            })
            w := call(a, "GET", "/api/search?q=日本&api_key=bad", "")
            if count != 1 || w.Code != 200 || !strings.Contains(w.Body.String(), `"page":1`) {
                t.Fatal(w.Body.String())
            }
        })
    }
}

func TestRecommendationSelection(t *testing.T) {
    mood := "暖かい気持ちが残る邦画"
    input := recommendationInput{Mood: &mood, Library: []savedMovie{
        {movie: movie{ID: 10, Title: "好きな作品"}, Status: "watched", Rating: 5, Note: "よかった"},
        {movie: movie{ID: 20, Demo: true}, Status: "want"},
    }}
    answer := `{"answers":{"movie":{"choice":"20","confidence":0.8}}}`
    tmdbBody := `{"results":[{"id":10,"title":"登録済み"},{"id":20,"title":"未登録","overview":"家族の物語","poster_path":"/poster.jpg"}]}`
    expectedPath := "/3/discover/movie"
    a := testApp(t, config{TMDBToken: "token", JevKey: "key"}, func(r *http.Request) (*http.Response, error) {
        if r.URL.Host == "api.themoviedb.org" {
            if r.URL.Path != expectedPath {
                t.Fatalf("got path %s", r.URL.Path)
            }
            if expectedPath == "/3/discover/movie" && (r.URL.Query().Get("with_origin_country") != "JP" || r.URL.Query().Get("with_original_language") != "ja") {
                t.Fatal("missing Japanese filters")
            }
            return jsonResponse(tmdbBody, 200), nil
        }
        if r.URL.String() != "https://api.typesafe.ai/v1/systemone" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer key" {
            t.Fatal("Jev request mismatch")
        }
        var payload struct {
            Model string `json:"model"`
            State struct {
                Mood    string           `json:"mood"`
                Watched []map[string]any `json:"watched"`
            } `json:"state"`
            Questions map[string]struct {
                Criteria map[string]json.RawMessage `json:"criteria"`
            } `json:"questions"`
        }
        if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
            t.Fatal(err)
        }
        criteria := payload.Questions["movie"].Criteria
        if payload.Model != "jev-latest" || payload.State.Mood != strings.TrimSpace(mood) || len(payload.State.Watched) != 1 || criteria["10"] != nil || criteria["20"] == nil || criteria["none"] == nil {
            t.Fatal("incorrect Jev payload")
        }
        return jsonResponse(answer, 200), nil
    })
    result, err := a.recommend(context.Background(), input)
    if err != nil || result.Movie.ID != 20 || result.Confidence != 0.8 || result.Movie.PosterPath == nil {
        t.Fatalf("got %+v %v", result, err)
    }
    body, err := json.Marshal(input)
    if err != nil {
        t.Fatal(err)
    }
    response := call(a, "POST", "/api/recommend", string(body))
    var decoded recommendation
    if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil || response.Code != 200 || decoded.Movie.ID != 20 || decoded.Confidence != 0.8 {
        t.Fatalf("HTTP recommendation: %d %s", response.Code, response.Body.String())
    }
    mood = "冒険したい"
    expectedPath = "/3/movie/10/recommendations"
    if _, err := a.recommend(context.Background(), input); err != nil {
        t.Fatal(err)
    }
    mood = "邦画以外が見たい"
    if _, err := a.recommend(context.Background(), input); err != nil {
        t.Fatal(err)
    }
    input.Library[0].Rating = 3
    expectedPath = "/3/movie/popular"
    if _, err := a.recommend(context.Background(), input); err != nil {
        t.Fatal(err)
    }
    for _, tc := range []struct{ answer, message string }{
        {`{"answers":{"movie":{"choice":"none","confidence":0.8}}}`, "今回の候補"},
        {`{"answers":{"movie":{"choice":"999","confidence":0.8}}}`, invalidAnswer},
        {`{}`, invalidAnswer},
        {`{"answers":{"movie":{"choice":"20"}}}`, invalidAnswer},
        {`{"answers":{"movie":{"choice":"20","confidence":null}}}`, invalidAnswer},
        {`{"answers":{"movie":{"choice":"20","confidence":1.1}}}`, invalidAnswer},
        {`{"answers":{"movie":{"choice":"20","confidence":-0.1}}}`, invalidAnswer},
    } {
        answer = tc.answer
        if _, err := a.recommend(context.Background(), input); err == nil || !strings.Contains(err.Error(), tc.message) {
            t.Fatalf("answer %s: %v", answer, err)
        }
    }
    tmdbBody = `{"results":[]}`
    if _, err := a.recommend(context.Background(), input); err == nil || !strings.Contains(err.Error(), "候補が見つかりません") {
        t.Fatal(err)
    }
}

func TestCandidateLimitAndInputLimits(t *testing.T) {
    candidates := make([]movie, 20)
    for i := range candidates {
        candidates[i] = movie{ID: int64(i + 1), Title: "作品"}
    }
    encoded, _ := json.Marshal(map[string]any{"results": candidates})
    a := testApp(t, config{TMDBToken: "token", JevKey: "key"}, func(r *http.Request) (*http.Response, error) {
        if r.URL.Host == "api.themoviedb.org" {
            return jsonResponse(string(encoded), 200), nil
        }
        var payload map[string]any
        _ = json.NewDecoder(r.Body).Decode(&payload)
        criteria := payload["questions"].(map[string]any)["movie"].(map[string]any)["criteria"].(map[string]any)
        if len(criteria) != 16 || criteria["15"] == nil || criteria["16"] != nil {
            t.Fatal("candidate limit")
        }
        return jsonResponse(`{"answers":{"movie":{"choice":"1","confidence":0}}}`, 200), nil
    })
    mood := "気分"
    input := recommendationInput{Mood: &mood, Library: []savedMovie{}}
    if _, err := a.recommend(context.Background(), input); err != nil {
        t.Fatal(err)
    }
    mood = strings.Repeat("😀", 251)
    if _, err := a.recommend(context.Background(), input); err == nil || err.Error() != invalidInput {
        t.Fatal(err)
    }
    mood = "気分"
    input.Library = make([]savedMovie, 501)
    if _, err := a.recommend(context.Background(), input); err == nil || err.Error() != invalidInput {
        t.Fatal(err)
    }
}

func TestExternalAPIFailure(t *testing.T) {
    for _, status := range []int{401, 429, 500} {
        a := testApp(t, config{TMDBToken: "token"}, func(r *http.Request) (*http.Response, error) { return jsonResponse(`{"secret":"hidden"}`, status), nil })
        w := call(a, "GET", "/api/search", "")
        if w.Code != 400 || !strings.Contains(w.Body.String(), externalError) || strings.Contains(w.Body.String(), "hidden") {
            t.Fatal(w.Body.String())
        }
    }
}
