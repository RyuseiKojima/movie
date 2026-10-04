package main

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "regexp"
    "strconv"
    "strings"
    "unicode/utf16"
)

const externalError = "外部APIへの接続に失敗しました。APIキーと利用状況をご確認ください。"
const invalidInput = "気分または登録作品の形式が不正です。"
const invalidAnswer = "Jevから有効な選択結果を取得できませんでした。時間をおいて再度お試しください。"

var apiKeyPattern = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)
var japanesePattern = regexp.MustCompile(`邦画|日本映画|日本の映画`)
var excludeJapanesePattern = regexp.MustCompile(`(?:邦画|日本映画|日本の映画)(?:以外|ではなく|じゃなく|は除|を除|は避|を避)`)

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
    Mood    *string      `json:"mood"`
    Library []savedMovie `json:"library"`
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
    if a.cfg.TMDBToken == "" || a.cfg.JevKey == "" {
        return output, errors.New("提案にはTMDBとJevのAPIキーが必要です。.envを設定してください。")
    }
    watched := make([]map[string]any, 0)
    registered := make(map[int64]bool)
    var favorite *savedMovie
    for i := range input.Library {
        saved := &input.Library[i]
        if !saved.Demo {
            registered[saved.ID] = true
        }
        if saved.Status == "watched" {
            watched = append(watched, map[string]any{"title": saved.Title, "rating": saved.Rating, "note": saved.Note})
            if saved.Rating >= 4 && (favorite == nil || saved.Rating > favorite.Rating) {
                favorite = saved
            }
        }
    }
    path := "movie/popular"
    params := url.Values{}
    if japanesePattern.MatchString(mood) && !excludeJapanesePattern.MatchString(mood) {
        path = "discover/movie"
        params.Set("with_origin_country", "JP")
        params.Set("with_original_language", "ja")
        params.Set("sort_by", "popularity.desc")
    } else if favorite != nil && !favorite.Demo {
        path = fmt.Sprintf("movie/%d/recommendations", favorite.ID)
    }
    var data struct {
        Results []movie `json:"results"`
    }
    if err := a.tmdb(ctx, path, params, &data); err != nil {
        return output, err
    }
    candidates := make([]movie, 0, 15)
    criteria := map[string]any{"none": "すべての候補がユーザーの明示した希望に反している"}
    for _, candidate := range data.Results {
        if registered[candidate.ID] {
            continue
        }
        candidates = append(candidates, candidate)
        criteria[strconv.FormatInt(candidate.ID, 10)] = map[string]string{"title": candidate.Title, "overview": candidate.Overview}
        if len(candidates) == 15 {
            break
        }
    }
    if len(candidates) == 0 {
        return output, errors.New("候補が見つかりませんでした。別の作品を評価してお試しください。")
    }
    payload := map[string]any{
        "model": a.cfg.JevModel,
        "state": map[string]any{"mood": mood, "watched": watched},
        "questions": map[string]any{"movie": map[string]any{
            "type":         "choice",
            "instructions": "今の気分を最優先に、候補の中で最も合う映画を1本選んでください。鑑賞履歴、評価、メモは補助情報です。短い気分や抽象的な気分でも、タイトルとあらすじから比較してください。完全な一致は必要ありません。すべての候補がユーザーの明示した希望に反する場合にだけnoneを選んでください。",
            "criteria":     criteria,
        }},
    }
    var result struct {
        Answers map[string]struct {
            Choice     string   `json:"choice"`
            Confidence *float64 `json:"confidence"`
        } `json:"answers"`
    }
    if err := a.requestJSON(ctx, http.MethodPost, "https://api.typesafe.ai/v1/systemone", a.cfg.JevKey, payload, &result); err != nil {
        return output, err
    }
    answer := result.Answers["movie"]
    if answer.Choice == "none" {
        return output, errors.New("今回の候補には気分に合う作品がありませんでした。別の作品を評価するか、気分を変えてお試しください。")
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
