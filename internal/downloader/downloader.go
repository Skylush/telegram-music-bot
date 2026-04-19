package downloader

import (
    "bufio"
    "bytes"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "net/url"
    "strings"
    "time"

    "music_bot/internal/util"
)

type Downloader struct {
    client *http.Client
}

func New(timeoutSeconds int) *Downloader {
    return &Downloader{client: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}}
}

func (d *Downloader) Download(url, dir, displayName string, songID int64, quality string) (string, error) {
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return "", err
    }

    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return "", err
    }
    req.Header.Set("User-Agent", "Mozilla/5.0")
    if u, err := url2Host(url); err == nil {
        if strings.HasSuffix(u, ".kuwo.cn") || u == "kuwo.cn" {
            req.Header.Set("Referer", "https://www.kuwo.cn/")
        }
    }

    resp, err := d.client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("download failed, status %d", resp.StatusCode)
    }

    if resp.Request != nil && resp.Request.URL != nil {
        if strings.Contains(resp.Request.URL.Path, "/404") {
            return "", fmt.Errorf("song is unavailable for direct download")
        }
    }

    contentType := strings.ToLower(resp.Header.Get("Content-Type"))
    if strings.Contains(contentType, "text/html") {
        return "", fmt.Errorf("received html page instead of audio")
    }

    br := bufio.NewReader(resp.Body)
    header, _ := br.Peek(512)
    detected := strings.ToLower(http.DetectContentType(header))
    if strings.Contains(detected, "text/html") || strings.Contains(detected, "text/plain") {
        return "", fmt.Errorf("response is not audio stream")
    }

    ext := detectExt(header)
    fileName := fmt.Sprintf("%d_%s_%s%s", songID, util.SafeFilename(displayName), quality, ext)
    dst := filepath.Join(dir, fileName)

    file, err := os.Create(dst)
    if err != nil {
        return "", err
    }
    defer file.Close()

    if _, err := io.Copy(file, br); err != nil {
        return "", err
    }

    return dst, nil
}

func url2Host(raw string) (string, error) {
    u, err := url.Parse(raw)
    if err != nil {
        return "", err
    }
    return strings.ToLower(u.Hostname()), nil
}

func detectExt(header []byte) string {
    if len(header) >= 4 {
        if bytes.Equal(header[:4], []byte("fLaC")) {
            return ".flac"
        }
        if bytes.Equal(header[:3], []byte("ID3")) {
            return ".mp3"
        }
    }
    if len(header) >= 2 {
        if header[0] == 0xFF && (header[1]&0xE0) == 0xE0 {
            return ".mp3"
        }
    }
    return ".mp3"
}
