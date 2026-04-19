package config

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "strconv"

    "gopkg.in/yaml.v3"
)

type Config struct {
    BotToken           string   `yaml:"-"`
    BotTokenFile       string   `yaml:"bot_token_file"`
    DownloadDir        string   `yaml:"download_dir"`
    MaxResults         int      `yaml:"max_results"`
    HTTPTimeoutSeconds int      `yaml:"http_timeout_seconds"`
    HTTPMaxRetries     int      `yaml:"http_max_retries"`
    LogLevel           string   `yaml:"log_level"`
    SourceAPIBaseURL   string   `yaml:"source_api_base_url"`
    SourceOrder        []string `yaml:"source_order"`
    ConfigFile         string   `yaml:"-"`
}

type fileConfig struct {
    BotTokenFile       string   `yaml:"bot_token_file"`
    DownloadDir        string   `yaml:"download_dir"`
    MaxResults         int      `yaml:"max_results"`
    HTTPTimeoutSeconds int      `yaml:"http_timeout_seconds"`
    HTTPMaxRetries     int      `yaml:"http_max_retries"`
    LogLevel           string   `yaml:"log_level"`
    SourceAPIBaseURL   string   `yaml:"source_api_base_url"`
    SourceOrder        []string `yaml:"source_order"`
}

func Load() (Config, error) {
    cfg := Config{
        DownloadDir:        "./data/downloads",
        MaxResults:         5,
        HTTPTimeoutSeconds: 20,
        HTTPMaxRetries:     2,
        LogLevel:           "info",
        SourceAPIBaseURL:   "https://music-api.gdstudio.xyz/api.php",
        SourceOrder:        []string{"netease", "kuwo", "joox"},
    }

    if configPath := resolveConfigPath(); configPath != "" {
        if err := loadConfigFile(configPath, &cfg); err == nil {
            cfg.ConfigFile = configPath
        } else if !errors.Is(err, os.ErrNotExist) {
            return Config{}, err
        }
    }

    if token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")); token != "" {
        cfg.BotToken = token
    }
    if tokenFile := strings.TrimSpace(envOr("BOT_TOKEN_FILE", cfg.BotTokenFile)); tokenFile != "" {
        cfg.BotTokenFile = tokenFile
    }
    if cfg.BotToken == "" && cfg.BotTokenFile != "" {
        tokenBytes, err := os.ReadFile(cfg.BotTokenFile)
        if err != nil {
            return Config{}, fmt.Errorf("read bot token file %s: %w", cfg.BotTokenFile, err)
        }
        cfg.BotToken = strings.TrimSpace(string(tokenBytes))
    }

    cfg.DownloadDir = envOr("DOWNLOAD_DIR", cfg.DownloadDir)
    cfg.LogLevel = envOr("LOG_LEVEL", cfg.LogLevel)
    cfg.SourceAPIBaseURL = envOr("SOURCE_API_BASE_URL", cfg.SourceAPIBaseURL)

    if v := envOr("SOURCE_ORDER", ""); v != "" {
        cfg.SourceOrder = parseList(v)
    }
    if v := envOr("SOURCE_ORDER", ""); v == "" && len(cfg.SourceOrder) == 0 {
        cfg.SourceOrder = []string{"netease", "kuwo", "joox"}
    }
    if cfg.MaxResults == 0 {
        cfg.MaxResults = envIntOr("MAX_RESULTS", 5)
    } else {
        cfg.MaxResults = envIntOr("MAX_RESULTS", cfg.MaxResults)
    }
    if cfg.HTTPTimeoutSeconds == 0 {
        cfg.HTTPTimeoutSeconds = envIntOr("HTTP_TIMEOUT_SECONDS", 20)
    } else {
        cfg.HTTPTimeoutSeconds = envIntOr("HTTP_TIMEOUT_SECONDS", cfg.HTTPTimeoutSeconds)
    }
    if cfg.HTTPMaxRetries == 0 && envOr("HTTP_MAX_RETRIES", "") == "" {
        cfg.HTTPMaxRetries = 2
    } else {
        cfg.HTTPMaxRetries = envIntOr("HTTP_MAX_RETRIES", cfg.HTTPMaxRetries)
    }

    if cfg.BotToken == "" {
        return Config{}, errors.New("TELEGRAM_BOT_TOKEN or BOT_TOKEN_FILE is required")
    }
    if cfg.MaxResults <= 0 || cfg.MaxResults > 20 {
        return Config{}, fmt.Errorf("MAX_RESULTS must be between 1 and 20, got %d", cfg.MaxResults)
    }
    if cfg.HTTPTimeoutSeconds <= 0 || cfg.HTTPTimeoutSeconds > 120 {
        return Config{}, fmt.Errorf("HTTP_TIMEOUT_SECONDS must be between 1 and 120, got %d", cfg.HTTPTimeoutSeconds)
    }
    if cfg.HTTPMaxRetries < 0 || cfg.HTTPMaxRetries > 10 {
        return Config{}, fmt.Errorf("HTTP_MAX_RETRIES must be between 0 and 10, got %d", cfg.HTTPMaxRetries)
    }
    cfg.SourceOrder = normalizeList(cfg.SourceOrder)
    if len(cfg.SourceOrder) == 0 {
        cfg.SourceOrder = []string{"netease", "kuwo", "joox"}
    }
    if strings.TrimSpace(cfg.SourceAPIBaseURL) == "" {
        cfg.SourceAPIBaseURL = "https://music-api.gdstudio.xyz/api.php"
    }

    return cfg, nil
}

func loadConfigFile(path string, cfg *Config) error {
    raw, err := os.ReadFile(path)
    if err != nil {
        return err
    }

    var fc fileConfig
    if err := yaml.Unmarshal(raw, &fc); err != nil {
        return fmt.Errorf("parse config file %s: %w", path, err)
    }

    if fc.BotTokenFile != "" {
        cfg.BotTokenFile = fc.BotTokenFile
    }
    if fc.DownloadDir != "" {
        cfg.DownloadDir = fc.DownloadDir
    }
    if fc.MaxResults != 0 {
        cfg.MaxResults = fc.MaxResults
    }
    if fc.HTTPTimeoutSeconds != 0 {
        cfg.HTTPTimeoutSeconds = fc.HTTPTimeoutSeconds
    }
    if fc.HTTPMaxRetries != 0 {
        cfg.HTTPMaxRetries = fc.HTTPMaxRetries
    }
    if fc.LogLevel != "" {
        cfg.LogLevel = fc.LogLevel
    }
    if fc.SourceAPIBaseURL != "" {
        cfg.SourceAPIBaseURL = fc.SourceAPIBaseURL
    }
    if len(fc.SourceOrder) != 0 {
        cfg.SourceOrder = fc.SourceOrder
    }
    return nil
}

func resolveConfigPath() string {
    if path := strings.TrimSpace(os.Getenv("CONFIG_FILE")); path != "" {
        return path
    }
    candidates := []string{
        "./config/config.yaml",
        "/app/config/config.yaml",
    }
    for _, candidate := range candidates {
        if _, err := os.Stat(candidate); err == nil {
            if abs, err := filepath.Abs(candidate); err == nil {
                return abs
            }
            return candidate
        }
    }
    return ""
}

func parseList(v string) []string {
    parts := strings.FieldsFunc(v, func(r rune) bool {
        return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
    })
    return normalizeList(parts)
}

func normalizeList(values []string) []string {
    seen := make(map[string]struct{}, len(values))
    result := make([]string, 0, len(values))
    for _, value := range values {
        item := strings.ToLower(strings.TrimSpace(value))
        if item == "" {
            continue
        }
        if _, ok := seen[item]; ok {
            continue
        }
        seen[item] = struct{}{}
        result = append(result, item)
    }
    return result
}

func envOr(key, def string) string {
    v := os.Getenv(key)
    if v == "" {
        return def
    }
    return v
}

func envIntOr(key string, def int) int {
    v := os.Getenv(key)
    if v == "" {
        return def
    }
    i, err := strconv.Atoi(v)
    if err != nil {
        return def
    }
    return i
}
