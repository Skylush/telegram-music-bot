package bot

import (
    "context"
    "fmt"
    "log/slog"
    "path/filepath"
    "os"
    "strconv"
    "strings"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

    "music_bot/internal/config"
    "music_bot/internal/downloader"
    "music_bot/internal/music"
)

type App struct {
    cfg          config.Config
    logger       *slog.Logger
    bot          *tgbotapi.BotAPI
    provider     music.Provider
    gdClient     *music.GDSourceClient
    downloader   *downloader.Downloader
    searchCache  map[int64]map[int64]music.Song
    chatPaths    map[int64]string
    chatQuality  map[int64]string
    sourceOrder  []string
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
    tgBot, err := tgbotapi.NewBotAPI(cfg.BotToken)
    if err != nil {
        return nil, err
    }

    return &App{
        cfg:         cfg,
        logger:      logger,
        bot:         tgBot,
        provider:    music.NewNeteaseProvider(cfg.HTTPTimeoutSeconds, cfg.HTTPMaxRetries),
        gdClient:    music.NewGDSourceClientWithBaseURL(cfg.HTTPTimeoutSeconds, cfg.HTTPMaxRetries, cfg.SourceAPIBaseURL),
        downloader:  downloader.New(cfg.HTTPTimeoutSeconds),
        searchCache: make(map[int64]map[int64]music.Song),
        chatPaths:   make(map[int64]string),
        chatQuality: make(map[int64]string),
        sourceOrder: append([]string{}, cfg.SourceOrder...),
    }, nil
}

func (a *App) Run(ctx context.Context) error {
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 30

    updates := a.bot.GetUpdatesChan(u)

    for {
        select {
        case <-ctx.Done():
            return nil
        case update := <-updates:
            if update.Message != nil {
                a.handleMessage(update.Message)
            }
            if update.CallbackQuery != nil {
                a.handleCallback(update.CallbackQuery)
            }
        }
    }
}

func (a *App) handleMessage(msg *tgbotapi.Message) {
    cmd := ParseCommand(msg.Text)
    switch cmd.Name {
    case "start", "help":
        a.sendText(msg.Chat.ID, "可用指令:\n/search 关键词 - 搜索歌曲并选择音质下载\n/download 歌曲ID - 用默认音质下载\n/quality [128|192|320|999] - 查看或设置默认音质\n/path - 选择保存目录\n/setpath 目录 - 设置自定义保存目录\n/where - 查看当前保存目录\n")
    case "search":
        if cmd.Arg == "" {
            a.sendText(msg.Chat.ID, "用法: /search 夜曲")
            return
        }
        songs, err := a.searchSongs(cmd.Arg)
        if err != nil {
            a.logger.Error("search failed", "error", err, "keyword", cmd.Arg)
            a.sendText(msg.Chat.ID, "搜索失败，请稍后再试")
            return
        }
        if len(songs) == 0 {
            a.sendText(msg.Chat.ID, "没有找到结果")
            return
        }

        rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(songs))
        if _, ok := a.searchCache[msg.Chat.ID]; !ok {
            a.searchCache[msg.Chat.ID] = make(map[int64]music.Song)
        }
        for _, song := range songs {
            text := fmt.Sprintf("%s - %s", song.Name, song.Artists)
            a.searchCache[msg.Chat.ID][song.ID] = song
            rows = append(rows, tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData(text, "pick:"+strconv.FormatInt(song.ID, 10)),
            ))
        }
        m := tgbotapi.NewMessage(msg.Chat.ID, "请选择要下载的歌曲:")
        m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
        _, _ = a.bot.Send(m)
    case "download":
        if cmd.Arg == "" {
            a.sendText(msg.Chat.ID, "用法: /download 123456")
            return
        }
        songID, err := strconv.ParseInt(strings.TrimSpace(cmd.Arg), 10, 64)
        if err != nil {
            a.sendText(msg.Chat.ID, "歌曲ID格式不正确")
            return
        }
        a.downloadSong(msg.Chat.ID, music.Song{ID: songID, Name: strconv.FormatInt(songID, 10), Source: "netease"}, a.getChatQuality(msg.Chat.ID))
    case "quality":
        if cmd.Arg != "" {
            if !isValidQuality(cmd.Arg) {
                a.sendText(msg.Chat.ID, "音质仅支持: 128/192/320/999")
                return
            }
            a.chatQuality[msg.Chat.ID] = cmd.Arg
            a.sendText(msg.Chat.ID, fmt.Sprintf("默认音质已设置为 %s", qualityLabel(cmd.Arg)))
            return
        }
        a.sendQualityMenu(msg.Chat.ID, 0)
    case "path":
        a.sendPathMenu(msg.Chat.ID)
    case "setpath":
        if cmd.Arg == "" {
            a.sendText(msg.Chat.ID, "用法: /setpath 目录路径")
            return
        }
        a.chatPaths[msg.Chat.ID] = a.resolveDownloadDir(msg.Chat.ID, cmd.Arg)
        a.sendText(msg.Chat.ID, fmt.Sprintf("保存目录已设置为: %s", a.chatPaths[msg.Chat.ID]))
    case "where":
        a.sendText(msg.Chat.ID, fmt.Sprintf("当前保存目录: %s", a.getChatDownloadDir(msg.Chat.ID)))
    default:
        if strings.HasPrefix(strings.TrimSpace(msg.Text), "/") {
            a.sendText(msg.Chat.ID, "未知指令，输入 /help 查看帮助")
        }
    }
}

func (a *App) handleCallback(cb *tgbotapi.CallbackQuery) {
    if cb.Message == nil {
        return
    }

    data := cb.Data
    switch {
    case strings.HasPrefix(data, "pick:"):
        payload := strings.TrimPrefix(data, "pick:")
        songID, err := strconv.ParseInt(payload, 10, 64)
        if err != nil {
            return
        }
        a.sendQualityMenu(cb.Message.Chat.ID, songID)
        ack := tgbotapi.NewCallback(cb.ID, "请选择音质")
        _, _ = a.bot.Request(ack)
        return
    case strings.HasPrefix(data, "dl:"):
        payload := strings.TrimPrefix(data, "dl:")
        parts := strings.SplitN(payload, ":", 2)
        songID, err := strconv.ParseInt(parts[0], 10, 64)
        if err != nil {
            return
        }

        quality := a.getChatQuality(cb.Message.Chat.ID)
        if len(parts) > 1 && isValidQuality(parts[1]) {
            quality = parts[1]
        }

        songMeta := music.Song{ID: songID, Name: strconv.FormatInt(songID, 10), Source: "netease"}
        if byChat, ok := a.searchCache[cb.Message.Chat.ID]; ok {
            if cachedSong, found := byChat[songID]; found {
                songMeta = cachedSong
            }
        }

        a.downloadSong(cb.Message.Chat.ID, songMeta, quality)

        ack := tgbotapi.NewCallback(cb.ID, "已收到下载请求")
        _, _ = a.bot.Request(ack)
        return
    case strings.HasPrefix(data, "setq:"):
        quality := strings.TrimPrefix(data, "setq:")
        if !isValidQuality(quality) {
            return
        }
        a.chatQuality[cb.Message.Chat.ID] = quality
        ack := tgbotapi.NewCallback(cb.ID, "默认音质已更新")
        _, _ = a.bot.Request(ack)
        a.sendText(cb.Message.Chat.ID, fmt.Sprintf("默认音质已设置为 %s", qualityLabel(quality)))
        return
    case strings.HasPrefix(data, "setpath:"):
        key := strings.TrimPrefix(data, "setpath:")
        path := a.quickPath(cb.Message.Chat.ID, key)
        a.chatPaths[cb.Message.Chat.ID] = path
        ack := tgbotapi.NewCallback(cb.ID, "保存目录已更新")
        _, _ = a.bot.Request(ack)
        a.sendText(cb.Message.Chat.ID, fmt.Sprintf("保存目录已设置为: %s", path))
        return
    }
}

func (a *App) downloadSong(chatID int64, song music.Song, quality string) {
    a.sendText(chatID, "开始下载，请稍候...")
    fileTitle := displaySongTitle(song)

    primaryURL, err := a.resolveSongURL(song, quality)
    if err != nil {
        a.logger.Warn("primary source resolve failed, trying fallback sources", "song_id", song.ID, "error", err)
        primaryURL = ""
    }

    if primaryURL == "" && strings.EqualFold(song.Source, "netease") {
        primaryURL = a.provider.DownloadURL(song.ID, quality)
    }

    var dst string
    if primaryURL != "" {
        dst, err = a.downloader.Download(primaryURL, a.getChatDownloadDir(chatID), fileTitle, song.ID, quality)
    } else {
        err = fmt.Errorf("no primary url resolved")
    }
    if err != nil {
        a.logger.Warn("primary source failed, trying fallback sources", "song_id", song.ID, "error", err)
        dst, err = a.downloadWithFallback(chatID, song, fileTitle, quality)
    }
    if err != nil {
        a.logger.Error("download failed", "error", err, "song_id", song.ID)
        a.sendText(chatID, "下载失败：网易云与备用音源都没有可用直链，请尝试其他歌曲或更低音质")
        return
    }

    sizeText := formatFileSize(dst)
    if sizeText != "" {
        a.sendText(chatID, fmt.Sprintf("下载完成: %s (大小: %s)", dst, sizeText))
        return
    }
    a.sendText(chatID, fmt.Sprintf("下载完成: %s", dst))
}

func (a *App) downloadWithFallback(chatID int64, song music.Song, fileTitle string, quality string) (string, error) {
    keyword := strings.TrimSpace(song.Name + " " + song.Artists)
    sources := a.fallbackSourceOrder(song.Source)
    var lastErr error

    for _, source := range sources {
        candidates, err := a.gdClient.Search(keyword, source, 3)
        if err != nil {
            lastErr = err
            continue
        }
        for _, candidate := range candidates {
            url, err := a.gdClient.ResolveURL(candidate, quality)
            if err != nil {
                lastErr = err
                continue
            }
            dst, err := a.downloader.Download(url, a.getChatDownloadDir(chatID), fileTitle, song.ID, quality)
            if err == nil {
                a.sendText(chatID, fmt.Sprintf("已自动切换到备用音源: %s", strings.ToUpper(candidate.Source)))
                return dst, nil
            }
            lastErr = err
        }
    }

    if lastErr == nil {
        lastErr = fmt.Errorf("no fallback source succeeded")
    }
    return "", lastErr
}

func (a *App) searchSongs(keyword string) ([]music.Song, error) {
    sources := a.sourceOrder
    if len(sources) == 0 {
        sources = []string{"netease", "kuwo", "joox"}
    }

    var lastErr error
    for _, source := range sources {
        if strings.EqualFold(source, "netease") {
            songs, err := a.provider.Search(keyword, a.cfg.MaxResults)
            if err == nil && len(songs) > 0 {
                return songs, nil
            }
            if err != nil {
                lastErr = err
            }
            continue
        }

        songs, err := a.gdClient.Search(keyword, source, a.cfg.MaxResults)
        if err != nil {
            lastErr = err
            continue
        }
        if len(songs) > 0 {
            return songs, nil
        }
    }

    if lastErr != nil {
        return nil, lastErr
    }
    return nil, fmt.Errorf("no songs found")
}

func (a *App) resolveSongURL(song music.Song, quality string) (string, error) {
    if strings.EqualFold(song.Source, "netease") || song.Source == "" {
        return a.provider.DownloadURL(song.ID, quality), nil
    }
    return a.gdClient.ResolveURL(song, quality)
}

func (a *App) fallbackSourceOrder(current string) []string {
    current = strings.ToLower(strings.TrimSpace(current))
    ordered := make([]string, 0, len(a.sourceOrder))
    seen := make(map[string]struct{}, len(a.sourceOrder))
    for _, source := range a.sourceOrder {
        normalized := strings.ToLower(strings.TrimSpace(source))
        if normalized == "" || normalized == current {
            continue
        }
        if _, ok := seen[normalized]; ok {
            continue
        }
        seen[normalized] = struct{}{}
        ordered = append(ordered, normalized)
    }
    if current != "" {
        if _, ok := seen[current]; !ok {
            ordered = append(ordered, current)
        }
    }
    if len(ordered) == 0 {
        return []string{"kuwo", "joox", "netease"}
    }
    return ordered
}

func displaySongTitle(song music.Song) string {
    name := strings.TrimSpace(song.Name)
    artists := strings.TrimSpace(song.Artists)
    if name == "" {
        name = strconv.FormatInt(song.ID, 10)
    }
    if artists == "" {
        return name
    }

    normalizedArtists := strings.NewReplacer("／", "&", "/", "&", ",", "&", " & ", "&").Replace(artists)
    normalizedArtists = strings.TrimSpace(normalizedArtists)
    normalizedArtists = strings.Trim(normalizedArtists, "&")
    return fmt.Sprintf("%s - %s", name, normalizedArtists)
}

func formatFileSize(path string) string {
    info, err := os.Stat(path)
    if err != nil {
        return ""
    }
    size := info.Size()
    if size >= 1024*1024*1024 {
        return fmt.Sprintf("%.2f GB", float64(size)/1024/1024/1024)
    }
    if size >= 1024*1024 {
        return fmt.Sprintf("%.2f MB", float64(size)/1024/1024)
    }
    if size >= 1024 {
        return fmt.Sprintf("%.2f KB", float64(size)/1024)
    }
    return fmt.Sprintf("%d B", size)
}

func (a *App) sendText(chatID int64, text string) {
    m := tgbotapi.NewMessage(chatID, text)
    _, _ = a.bot.Send(m)
}

func (a *App) sendQualityMenu(chatID int64, songID int64) {
    m := tgbotapi.NewMessage(chatID, "请选择音质:")
    m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("标准 128k", callbackForQuality(songID, "128")),
            tgbotapi.NewInlineKeyboardButtonData("高品 192k", callbackForQuality(songID, "192")),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("极高 320k", callbackForQuality(songID, "320")),
            tgbotapi.NewInlineKeyboardButtonData("无损 FLAC", callbackForQuality(songID, "999")),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("设为默认 128", "setq:128"),
            tgbotapi.NewInlineKeyboardButtonData("设为默认 320", "setq:320"),
        ),
    )
    _, _ = a.bot.Send(m)
}

func callbackForQuality(songID int64, quality string) string {
    if songID <= 0 {
        return "setq:" + quality
    }
    return fmt.Sprintf("dl:%d:%s", songID, quality)
}

func (a *App) sendPathMenu(chatID int64) {
    m := tgbotapi.NewMessage(chatID, fmt.Sprintf("当前保存目录: %s\n可选择快捷目录或使用 /setpath 自定义", a.getChatDownloadDir(chatID)))
    m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("默认目录", "setpath:default"),
            tgbotapi.NewInlineKeyboardButtonData("标准音质目录", "setpath:std"),
            tgbotapi.NewInlineKeyboardButtonData("无损目录", "setpath:lossless"),
        ),
    )
    _, _ = a.bot.Send(m)
}

func (a *App) getChatDownloadDir(chatID int64) string {
    if p, ok := a.chatPaths[chatID]; ok && strings.TrimSpace(p) != "" {
        return p
    }
    return a.cfg.DownloadDir
}

func (a *App) resolveDownloadDir(chatID int64, input string) string {
    p := filepath.Clean(strings.TrimSpace(input))
    if filepath.IsAbs(p) {
        return p
    }
    return filepath.Join(a.getChatDownloadDir(chatID), p)
}

func (a *App) quickPath(chatID int64, key string) string {
    switch key {
    case "std":
        return filepath.Join(a.cfg.DownloadDir, "standard")
    case "lossless":
        return filepath.Join(a.cfg.DownloadDir, "lossless")
    default:
        return a.cfg.DownloadDir
    }
}

func (a *App) getChatQuality(chatID int64) string {
    if q, ok := a.chatQuality[chatID]; ok && isValidQuality(q) {
        return q
    }
    return "320"
}

func qualityLabel(q string) string {
    switch q {
    case "128":
        return "标准 128k"
    case "192":
        return "高品 192k"
    case "999":
        return "无损 FLAC"
    default:
        return "极高 320k"
    }
}

func isValidQuality(q string) bool {
    return q == "128" || q == "192" || q == "320" || q == "999"
}
