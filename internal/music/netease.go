package music

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "time"
)

type NeteaseProvider struct {
    client     *http.Client
    maxRetries int
}

func NewNeteaseProvider(timeoutSeconds, maxRetries int) *NeteaseProvider {
    return &NeteaseProvider{
        client: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
        maxRetries: maxRetries,
    }
}

func (p *NeteaseProvider) Search(keyword string, limit int) ([]Song, error) {
    form := url.Values{}
    form.Set("s", keyword)
    form.Set("type", "1")
    form.Set("offset", "0")
    form.Set("limit", strconv.Itoa(limit))

    req, err := http.NewRequest(http.MethodPost, "https://music.163.com/api/cloudsearch/pc", bytes.NewBufferString(form.Encode()))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Referer", "https://music.163.com/")
    req.Header.Set("User-Agent", "Mozilla/5.0")

    var resp *http.Response
    for i := 0; i <= p.maxRetries; i++ {
        resp, err = p.client.Do(req.Clone(req.Context()))
        if err == nil {
            break
        }
    }
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("netease search failed, status %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var parsed struct {
        Result struct {
            Songs []struct {
                ID   int64  `json:"id"`
                Name string `json:"name"`
                Ar   []struct {
                    Name string `json:"name"`
                } `json:"ar"`
            } `json:"songs"`
        } `json:"result"`
    }
    if err := json.Unmarshal(body, &parsed); err != nil {
        return nil, err
    }

    songs := make([]Song, 0, len(parsed.Result.Songs))
    for _, s := range parsed.Result.Songs {
        artists := make([]string, 0, len(s.Ar))
        for _, a := range s.Ar {
            artists = append(artists, a.Name)
        }
        songs = append(songs, Song{ID: s.ID, Name: s.Name, Artists: strings.Join(artists, "/"), Source: "netease"})
    }

    return songs, nil
}

func (p *NeteaseProvider) DownloadURL(songID int64, quality string) string {
    br := "320000"
    switch quality {
    case "128":
        br = "128000"
    case "192":
        br = "192000"
    case "999":
        br = "999000"
    }
    return fmt.Sprintf("https://music.163.com/song/media/outer/url?id=%d.mp3&br=%s", songID, br)
}
