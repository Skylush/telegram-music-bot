package music

type Song struct {
    ID      int64
    Name    string
    Artists string
    Source  string
}

type Provider interface {
    Search(keyword string, limit int) ([]Song, error)
    DownloadURL(songID int64, quality string) string
}
