package main

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "net/url"
    "os"
    "path/filepath"
    "slices"
    "strconv"
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
        {"GET", "/api/search?yearTo=2023", "", 200, "雨の日の喫茶店"},
        {"GET", "/api/search?yearFrom=2025&yearTo=2025", "", 200, `"results":[{"id":3,`},
        {"GET", "/api/search?q=喫茶店&yearFrom=2024", "", 200, `"results":[]`},
        {"GET", "/api/search?yearFrom=abc", "", 400, "上映年"},
        {"GET", "/api/search?yearFrom=1879", "", 400, "上映年"},
        {"GET", "/api/search?yearFrom=2001&yearTo=2000", "", 400, "上映年"},
        {"GET", "/", "", 200, "Movie Shelf"},
        {"GET", "/unknown", "", 404, "見つかりません"},
        {"GET", "/../index.html", "", 404, "見つかりません"},
        {"POST", "/api/recommend", "{", 400, "入力形式"},
        {"POST", "/api/recommend", `{} {}`, 400, "入力形式"},
        {"POST", "/api/recommend", strings.Repeat("x", maxRecommendBytes+1), 413, "大きすぎ"},
        {"POST", "/api/recommend", `{"mood":"x","library":null}`, 400, "形式が不正"},
        {"POST", "/api/recommend", `{"mood":123,"library":[]}`, 400, "形式が不正"},
        {"POST", "/api/recommend", `{"mood":"   ","library":[]}`, 400, "今の気分"},
        {"POST", "/api/recommend", `{"mood":"x","library":[]}`, 400, "形式が不正"},
        {"POST", "/api/recommend", `{"mood":"x","library":[],"candidates":[]}`, 400, "APIキーが必要"},
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
            if count != 1 || w.Code != 200 || !strings.Contains(w.Body.String(), `"results":[]`) {
                t.Fatal(w.Body.String())
            }
        })
    }
}

func TestRecommendationSelection(t *testing.T) {
    mood := "  暖かい気持ちが残る作品  "
    input := recommendationInput{
        Mood: &mood,
        Library: []savedMovie{
            {movie: movie{ID: 10, Title: "好きな作品"}, Status: "watched", Rating: 5, Note: "よかった"},
            {movie: movie{ID: 20, Demo: true}, Status: "want"},
        },
        Candidates: []movie{
            {ID: 10, Title: "登録済み"},
            {ID: 20, Title: "未登録", Overview: strings.Repeat("家族の物語", 20), PosterPath: ptr("/poster.jpg")},
            {ID: 20, Title: "重複"},
        },
    }
    answer := `{"answers":{"movie":{"choice":"20","confidence":0.8}}}`
    a := testApp(t, config{JevKey: "key"}, func(r *http.Request) (*http.Response, error) {
        if r.URL.String() != "https://api.typesafe.ai/v1/systemone" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer key" {
            t.Fatalf("unexpected request %s", r.URL)
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
        var chosen struct{ Title, Overview string }
        _ = json.Unmarshal(criteria["20"], &chosen)
        if payload.Model != "jev-latest" || payload.State.Mood != strings.TrimSpace(mood) || len(payload.State.Watched) != 1 || len(criteria) != 2 || criteria["10"] != nil || criteria["none"] == nil {
            t.Fatalf("incorrect Jev payload %s", criteria)
        }
        if chosen.Title != "未登録" || len([]rune(chosen.Overview)) != maxOverviewRunes {
            t.Fatalf("incorrect candidate %+v", chosen)
        }
        return jsonResponse(answer, 200), nil
    })
    result, err := a.recommend(context.Background(), input)
    if err != nil || result.Movie.ID != 20 || result.Confidence != 0.8 || result.Movie.PosterPath == nil || len([]rune(result.Movie.Overview)) != 100 {
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
    for _, tc := range []struct{ answer, message string }{
        {`{"answers":{"movie":{"choice":"none","confidence":0.8}}}`, "今回の候補"},
        {`{"answers":{"movie":{"choice":"10","confidence":0.8}}}`, invalidAnswer},
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
    input.Candidates = input.Candidates[:1]
    if _, err := a.recommend(context.Background(), input); err == nil || !strings.Contains(err.Error(), "未登録の作品がありません") {
        t.Fatal(err)
    }
}

func ptr[T any](value T) *T { return &value }

func TestRecommendationInputLimits(t *testing.T) {
    a := testApp(t, config{JevKey: "key"}, func(r *http.Request) (*http.Response, error) {
        t.Fatal("Jev must not be called for invalid input")
        return nil, nil
    })
    mood := "気分"
    valid := []movie{{ID: 1, Title: "作品"}}
    for _, input := range []recommendationInput{
        {Mood: ptr(strings.Repeat("😀", 251)), Library: []savedMovie{}, Candidates: valid},
        {Mood: &mood, Library: make([]savedMovie, 501), Candidates: valid},
        {Mood: &mood, Library: []savedMovie{}},
        {Mood: &mood, Library: []savedMovie{}, Candidates: make([]movie, maxCandidates+1)},
        {Mood: &mood, Library: []savedMovie{}, Candidates: []movie{{ID: 1, Title: strings.Repeat("あ", 501)}}},
    } {
        if _, err := a.recommend(context.Background(), input); err == nil || err.Error() != invalidInput {
            t.Fatal(err)
        }
    }
}

// popularPages serves TMDB pages of 19 distinct movies plus one movie shared by every page.
func popularPages(t *testing.T, totalPages int, requested map[int]bool) func(*http.Request) *http.Response {
    return func(r *http.Request) *http.Response {
        page, _ := strconv.Atoi(r.URL.Query().Get("page"))
        if page < 1 || page > candidatePoolPages || requested[page] {
            t.Fatalf("unexpected page %d", page)
        }
        requested[page] = true
        movies := make([]movie, 0, 20)
        if page <= totalPages {
            for i := range 19 {
                movies = append(movies, movie{ID: int64(page*100 + i), Title: "作品", Overview: strings.Repeat("あ", 100)})
            }
            movies = append(movies, movie{ID: 1, Title: "共通"})
        }
        encoded, _ := json.Marshal(map[string]any{"results": movies, "total_pages": totalPages})
        return jsonResponse(string(encoded), 200)
    }
}

func TestRecommendationNoneWithSpreadProbabilities(t *testing.T) {
    var answer string
    a := testApp(t, config{JevKey: "key"}, func(r *http.Request) (*http.Response, error) {
        return jsonResponse(answer, 200), nil
    })
    mood := "悲しい"
    input := recommendationInput{Mood: &mood, Library: []savedMovie{}, Candidates: []movie{{ID: 1, Title: "A"}, {ID: 2, Title: "B"}, {ID: 3, Title: "C"}}}
    answer = `{"answers":{"movie":{"choice":"none","confidence":0.4,"probabilities":{"none":0.4,"1":0.15,"2":0.3,"3":0.15}}}}`
    result, err := a.recommend(context.Background(), input)
    if err != nil || result.Movie.ID != 2 || result.Confidence != 0.5 {
        t.Fatalf("got %+v %v", result, err)
    }
    for _, tc := range []struct{ answer, message string }{
        {`{"answers":{"movie":{"choice":"none","confidence":0.6,"probabilities":{"none":0.6,"1":0.2,"2":0.1,"3":0.1}}}}`, "今回の候補"},
        {`{"answers":{"movie":{"choice":"none","confidence":0.4}}}`, "今回の候補"},
        {`{"answers":{"movie":{"choice":"none","confidence":0.4,"probabilities":{"none":0.4,"999":0.6}}}}`, invalidAnswer},
    } {
        answer = tc.answer
        if _, err := a.recommend(context.Background(), input); err == nil || !strings.Contains(err.Error(), tc.message) {
            t.Fatalf("answer %s: %v", answer, err)
        }
    }
}

func TestDefaultSearchListsRandomCandidates(t *testing.T) {
    search := func(totalPages int) ([]movie, map[int]bool) {
        requested := map[int]bool{}
        tmdb := popularPages(t, totalPages, requested)
        a := testApp(t, config{TMDBToken: "token"}, func(r *http.Request) (*http.Response, error) {
            if r.URL.Path != "/3/movie/popular" {
                t.Fatalf("got path %s", r.URL.Path)
            }
            return tmdb(r), nil
        })
        w := call(a, "GET", "/api/search?q=", "")
        var data struct {
            Results []movie `json:"results"`
        }
        if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || w.Code != 200 {
            t.Fatalf("got %d %s", w.Code, w.Body.String())
        }
        seen := map[int64]bool{}
        for _, result := range data.Results {
            if seen[result.ID] {
                t.Fatalf("duplicate movie %d", result.ID)
            }
            seen[result.ID] = true
        }
        return data.Results, requested
    }
    results, requested := search(500)
    if len(results) != maxCandidates || len(requested) != 6 {
        t.Fatalf("got %d movies from %d pages", len(results), len(requested))
    }
    // Only the first request may land beyond the last page; after that, out-of-range pages are skipped.
    results, requested = search(2)
    if len(results) != 39 || !requested[1] || !requested[2] || len(requested) > 3 {
        t.Fatalf("got %d movies from pages %v", len(results), requested)
    }
    different := false
    for range 5 {
        other, _ := search(500)
        if !slices.EqualFunc(results, other, func(a, b movie) bool { return a.ID == b.ID }) {
            different = true
            break
        }
        results = other
    }
    if !different {
        t.Fatal("search results are not randomized")
    }
}

func TestSearchYearFilter(t *testing.T) {
    var path string
    var query url.Values
    a := testApp(t, config{TMDBToken: "token"}, func(r *http.Request) (*http.Response, error) {
        path, query = r.URL.Path, r.URL.Query()
        return jsonResponse(`{"results":[{"id":1,"title":"A","release_date":"1989-12-31"},{"id":2,"title":"B","release_date":"1995-05-01"},{"id":3,"title":"C"}],"total_pages":1}`, 200), nil
    })
    w := call(a, "GET", "/api/search?yearFrom=1990&yearTo=1999", "")
    if w.Code != 200 || path != "/3/discover/movie" || query.Get("sort_by") != "popularity.desc" || query.Get("primary_release_date.gte") != "1990-01-01" || query.Get("primary_release_date.lte") != "1999-12-31" {
        t.Fatalf("unexpected request %s %s: %s", path, query.Encode(), w.Body.String())
    }
    w = call(a, "GET", "/api/search?yearTo=1999", "")
    if path != "/3/discover/movie" || query.Has("primary_release_date.gte") || query.Get("primary_release_date.lte") != "1999-12-31" {
        t.Fatalf("unexpected query %s", query.Encode())
    }
    w = call(a, "GET", "/api/search?q=作品&yearFrom=1990&yearTo=1999", "")
    var data struct {
        Results []movie `json:"results"`
    }
    if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || path != "/3/search/movie" || len(data.Results) != 1 || data.Results[0].ID != 2 {
        t.Fatalf("got %s %s", path, w.Body.String())
    }
    w = call(a, "GET", "/api/search?q=作品", "")
    if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || len(data.Results) != 3 {
        t.Fatalf("got %s", w.Body.String())
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
