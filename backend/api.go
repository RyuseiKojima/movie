package main

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "io"
    "math/rand/v2"
    "net/http"
    "net/url"
    "regexp"
    "strconv"
    "strings"
    "unicode/utf16"
)

const externalError = "外部APIへの接続に失敗しました。APIキーと利用状況をご確認ください。"
const invalidInput = "気分・登録作品・候補の形式が不正です。"
const invalidAnswer = "Jevから有効な選択結果を取得できませんでした。時間をおいて再度お試しください。"
const invalidYear = "上映年は1880〜2100年の範囲で、開始年が終了年以前になるよう指定してください。"

const maxCandidates = 100
const maxCandidatePages = 10

// Candidates are sampled from the top TMDB pages so each request sees a different set.
const candidatePoolPages = 25

// Jev rejects long requests (max_tokens_exceeded), so overviews are shortened for many candidates.
const maxOverviewRunes = 40
const minYear = 1880
const maxYear = 2100

var apiKeyPattern = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)

type movie struct {
    ID          int64   `json:"id"`
    Title       string  `json:"title"`
    Overview    string  `json:"overview,omitempty"`
    ReleaseDate string  `json:"release_date,omitempty"`
    PosterPath  *string `json:"poster_path,omitempty"`
    GenreIDs    []int   `json:"genre_ids,omitempty"`
    VoteAverage float64 `json:"vote_average,omitempty"`
    Demo        bool    `json:"demo,omitempty"`
}

type savedMovie struct {
    movie
    Status string  `json:"status"`
    Rating float64 `json:"rating"`
    Note   string  `json:"note"`
}

type recommendationInput struct {
    Mood       *string      `json:"mood"`
    Library    []savedMovie `json:"library"`
    Candidates []movie      `json:"candidates"`
}

type recommendation struct {
    Movie      movie   `json:"movie"`
    Confidence float64 `json:"confidence"`
}

var demoMovies = []movie{
    {ID: 1, Title: "星をめぐる旅", Overview: "遠い星を目指す探検家と、地球で待つ家族の物語。", ReleaseDate: "2024-01-01", GenreIDs: []int{878}, VoteAverage: 8.2},
    {ID: 2, Title: "雨の日の喫茶店", Overview: "小さな喫茶店で出会った二人が、少しずつ新しい日常を見つける。", ReleaseDate: "2023-01-01", GenreIDs: []int{18}, VoteAverage: 7.8},
    {ID: 3, Title: "真夜中の手紙", Overview: "届くはずのない手紙を手がかりに、探偵が街の秘密を追う。", ReleaseDate: "2025-01-01", GenreIDs: []int{9648}, VoteAverage: 7.5},
}

// parseYearRange reads optional yearFrom/yearTo query values.
func parseYearRange(query url.Values) (from, to *int, err error) {
    years := make([]*int, 2)
    for i, name := range []string{"yearFrom", "yearTo"} {
        value := strings.TrimSpace(query.Get(name))
        if value == "" {
            continue
        }
        year, err := strconv.Atoi(value)
        if err != nil || year < minYear || year > maxYear {
            return nil, nil, errors.New(invalidYear)
        }
        years[i] = &year
    }
    if years[0] != nil && years[1] != nil && *years[0] > *years[1] {
        return nil, nil, errors.New(invalidYear)
    }
    return years[0], years[1], nil
}

func inYearRange(candidate movie, from, to *int) bool {
    if from == nil && to == nil {
        return true
    }
    year, err := strconv.Atoi(candidate.ReleaseDate[:min(4, len(candidate.ReleaseDate))])
    return err == nil && (from == nil || year >= *from) && (to == nil || year <= *to)
}

func truncateRunes(value string, limit int) string {
    runes := []rune(value)
    if len(runes) <= limit {
        return value
    }
    return string(runes[:limit])
}

// Preserve JavaScript's length limits, including two units for emoji.
func inputLength(value string) int {
    return len(utf16.Encode([]rune(value)))
}

func (a *app) requestJSON(ctx context.Context, method, endpoint, credential string, payload any, target any) error {
    var body io.Reader
    if payload != nil {
        encoded, err := json.Marshal(payload)
        if err != nil {
            return errors.New(externalError)
        }
        body = bytes.NewReader(encoded)
    }
    req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
    if err != nil {
        return errors.New(externalError)
    }
    if credential != "" {
        req.Header.Set("Authorization", "Bearer "+credential)
    }
    if payload != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    res, err := a.client.Do(req)
    if err != nil {
        return errors.New(externalError)
    }
    defer res.Body.Close()
    if res.StatusCode < 200 || res.StatusCode >= 300 {
        return errors.New(externalError)
    }
    if err := json.NewDecoder(io.LimitReader(res.Body, 5_000_000)).Decode(target); err != nil {
        return errors.New(externalError)
    }
    return nil
}

func (a *app) tmdb(ctx context.Context, path string, params url.Values, target any) error {
    if params == nil {
        params = url.Values{}
    }
    params.Set("language", "ja-JP")
    params.Set("include_adult", "false")
    credential := a.cfg.TMDBToken
    if apiKeyPattern.MatchString(credential) {
        params.Set("api_key", credential)
        credential = ""
    }
    return a.requestJSON(ctx, http.MethodGet, "https://api.themoviedb.org/3/"+path+"?"+params.Encode(), credential, nil, target)
}

func (a *app) recommend(ctx context.Context, input recommendationInput) (recommendation, error) {
    var output recommendation
    if input.Mood == nil || inputLength(*input.Mood) > 500 || input.Library == nil || len(input.Library) > 500 {
        return output, errors.New(invalidInput)
    }
    mood := strings.TrimSpace(*input.Mood)
    if mood == "" {
        return output, errors.New("今の気分を入力してください。")
    }
    if input.Candidates == nil || len(input.Candidates) > maxCandidates {
        return output, errors.New(invalidInput)
    }
    if a.cfg.JevKey == "" {
        return output, errors.New("提案にはJevのAPIキーが必要です。.envを設定してください。")
    }
    watched := make([]map[string]any, 0)
    registered := make(map[int64]bool)
    for _, saved := range input.Library {
        if !saved.Demo {
            registered[saved.ID] = true
        }
        if saved.Status == "watched" {
            watched = append(watched, map[string]any{"title": saved.Title, "rating": saved.Rating, "note": saved.Note})
        }
    }
    // Jev chooses from the movies shown in the list, except ones already on the shelf.
    candidates := make([]movie, 0, len(input.Candidates))
    criteria := map[string]any{"none": "すべての候補がユーザーの明示した希望に反している"}
    for _, candidate := range input.Candidates {
        key := strconv.FormatInt(candidate.ID, 10)
        if inputLength(candidate.Title) > 500 {
            return output, errors.New(invalidInput)
        }
        if registered[candidate.ID] || criteria[key] != nil {
            continue
        }
        candidates = append(candidates, candidate)
        criteria[key] = map[string]string{"title": candidate.Title, "overview": truncateRunes(candidate.Overview, maxOverviewRunes)}
    }
    if len(candidates) == 0 {
        return output, errors.New("一覧に未登録の作品がありません。検索や上映年の条件を変えてお試しください。")
    }
    payload := map[string]any{
        "model": a.cfg.JevModel,
        "state": map[string]any{"mood": mood, "watched": watched},
        "questions": map[string]any{"movie": map[string]any{
            "type":         "choice",
            "instructions": "今の気分を最優先に、候補の中で最も合う映画を1本選んでください。鑑賞履歴、評価、メモは補助情報です。短い気分や抽象的な気分でも、タイトルとあらすじから比較してください。「悲しい」「疲れた」のように感情だけが書かれている場合は、その気持ちに寄り添う作品か、気持ちを軽くする作品を選んでください。完全な一致は必要ありません。すべての候補がユーザーの明示した希望に反する場合にだけnoneを選んでください。",
            "criteria":     criteria,
        }},
    }
    var result struct {
        Answers map[string]struct {
            Choice        string             `json:"choice"`
            Confidence    *float64           `json:"confidence"`
            Probabilities map[string]float64 `json:"probabilities"`
        } `json:"answers"`
    }
    if err := a.requestJSON(ctx, http.MethodPost, "https://api.typesafe.ai/v1/systemone", a.cfg.JevKey, payload, &result); err != nil {
        return output, err
    }
    answer := result.Answers["movie"]
    if answer.Choice == "none" {
        // With many candidates, each movie gets a small share and "none" can win the argmax
        // even when the movies together are far more likely. Decline only when "none" is the majority.
        noneProbability, ok := answer.Probabilities["none"]
        if !ok || noneProbability >= 0.5 {
            return output, errors.New("今回の候補には気分に合う作品がありませんでした。別の作品を評価するか、気分を変えてお試しください。")
        }
        var best *movie
        bestProbability := -1.0
        for i := range candidates {
            probability := answer.Probabilities[strconv.FormatInt(candidates[i].ID, 10)]
            if probability > bestProbability {
                best, bestProbability = &candidates[i], probability
            }
        }
        confidence := bestProbability / (1 - noneProbability)
        if bestProbability <= 0 || confidence > 1 {
            return output, errors.New(invalidAnswer)
        }
        return recommendation{Movie: *best, Confidence: confidence}, nil
    }
    if answer.Confidence != nil && *answer.Confidence >= 0 && *answer.Confidence <= 1 {
        for _, candidate := range candidates {
            if strconv.FormatInt(candidate.ID, 10) == answer.Choice {
                return recommendation{Movie: candidate, Confidence: *answer.Confidence}, nil
            }
        }
    }
    return output, errors.New(invalidAnswer)
}

// fetchMovies collects up to maxCandidates unique movies from TMDB pages in random order
// and returns them shuffled.
func (a *app) fetchMovies(ctx context.Context, path string, params url.Values, keep func(movie) bool) ([]movie, error) {
    movies := make([]movie, 0, maxCandidates)
    seen := make(map[int64]bool)
    totalPages := candidatePoolPages
    requests := 0
    for _, index := range rand.Perm(candidatePoolPages) {
        if len(movies) >= maxCandidates || requests >= maxCandidatePages {
            break
        }
        page := index + 1
        if page > totalPages {
            continue
        }
        requests++
        params.Set("page", strconv.Itoa(page))
        var data struct {
            Results    []movie `json:"results"`
            TotalPages int     `json:"total_pages"`
        }
        if err := a.tmdb(ctx, path, params, &data); err != nil {
            // Keep the movies already collected when a later page fails.
            if requests == 1 {
                return nil, err
            }
            break
        }
        totalPages = min(totalPages, data.TotalPages)
        for _, candidate := range data.Results {
            if seen[candidate.ID] || !keep(candidate) {
                continue
            }
            seen[candidate.ID] = true
            movies = append(movies, candidate)
            if len(movies) == maxCandidates {
                break
            }
        }
    }
    rand.Shuffle(len(movies), func(i, j int) { movies[i], movies[j] = movies[j], movies[i] })
    return movies, nil
}
