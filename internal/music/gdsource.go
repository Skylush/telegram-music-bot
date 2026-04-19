package music

import (
    "encoding/json"
    "fmt"
    "io"
    "math/rand"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "time"
)

type GDSourceClient struct {
    client     *http.Client
    maxRetries int
    baseURL    string
}

func NewGDSourceClient(timeoutSeconds, maxRetries int) *GDSourceClient {
    return NewGDSourceClientWithBaseURL(timeoutSeconds, maxRetries, "https://music-api.gdstudio.xyz/api.php")
}

func NewGDSourceClientWithBaseURL(timeoutSeconds, maxRetries int, baseURL string) *GDSourceClient {
    return &GDSourceClient{
        client:     &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
        maxRetries: maxRetries,
        baseURL:    strings.TrimSpace(baseURL),
    }
}

func (c *GDSourceClient) Search(keyword, source string, limit int) ([]Song, error) {
    params := url.Values{}
    params.Set("types", "search")
    params.Set("source", source)
    params.Set("name", keyword)
    params.Set("count", strconv.Itoa(limit))
    params.Set("pages", "1")
    params.Set("s", randomSignature())

    endpoint := c.baseURL + "?" + params.Encode()

    var body []byte
    var err error
    for i := 0; i <= c.maxRetries; i++ {
        body, err = c.fetch(endpoint)
        if err == nil {
            break
        }
    }
    if err != nil {
        return nil, err
    }

    var arr []struct {
        ID     any    `json:"id"`
        Name   string `json:"name"`
        Artist any    `json:"artist"`
        Source string `json:"source"`
    }
    if err := json.Unmarshal(body, &arr); err != nil {
        return nil, err
    }

    songs := make([]Song, 0, len(arr))
    for _, item := range arr {
        id := parseAnyInt64(item.ID)
        if id <= 0 || strings.TrimSpace(item.Name) == "" {
            continue
        }
        songs = append(songs, Song{
            ID:      id,
            Name:    item.Name,
            Artists: parseArtist(item.Artist),
            Source:  normalizeSource(item.Source, source),
        })
    }
    return songs, nil
}

func (c *GDSourceClient) ResolveURL(song Song, quality string) (string, error) {
    params := url.Values{}
    params.Set("types", "url")
    params.Set("id", strconv.FormatInt(song.ID, 10))
    params.Set("source", normalizeSource(song.Source, "netease"))
    params.Set("br", quality)
    params.Set("s", randomSignature())

    endpoint := c.baseURL + "?" + params.Encode()

    var body []byte
    var err error
    for i := 0; i <= c.maxRetries; i++ {
        body, err = c.fetch(endpoint)
        if err == nil {
            break
        }
    }
    if err != nil {
        return "", err
    }

    var parsed struct {
        URL string `json:"url"`
    }
    if err := json.Unmarshal(body, &parsed); err != nil {
        return "", err
    }
    if strings.TrimSpace(parsed.URL) == "" {
        return "", fmt.Errorf("empty url from source %s", song.Source)
    }
    return parsed.URL, nil
}

func (c *GDSourceClient) fetch(endpoint string) ([]byte, error) {
    req, err := http.NewRequest(http.MethodGet, endpoint, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("User-Agent", "Mozilla/5.0")
    req.Header.Set("Accept", "application/json")

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("gd api status %d", resp.StatusCode)
    }

    return io.ReadAll(resp.Body)
}

func parseArtist(v any) string {
    switch t := v.(type) {
    case string:
        return t
    case []any:
        out := make([]string, 0, len(t))
        for _, x := range t {
            if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
                out = append(out, s)
            }
        }
        return strings.Join(out, "/")
    default:
        return ""
    }
}

func parseAnyInt64(v any) int64 {
    switch t := v.(type) {
    case float64:
        return int64(t)
    case int64:
        return t
    case string:
        i, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
        return i
    default:
        return 0
    }
}

func normalizeSource(source, fallback string) string {
    s := strings.ToLower(strings.TrimSpace(source))
    if s == "" {
        return fallback
    }
    return s
}

func randomSignature() string {
    return strconv.FormatInt(time.Now().UnixNano()+rand.Int63n(9999), 36)
}
