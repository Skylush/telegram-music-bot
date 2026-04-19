package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "music_bot/internal/bot"
    "music_bot/internal/config"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        panic(err)
    }

    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

    app, err := bot.New(cfg, logger)
    if err != nil {
        logger.Error("failed to initialize bot", "error", err)
        os.Exit(1)
    }

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    if err := app.Run(ctx); err != nil {
        logger.Error("bot stopped with error", "error", err)
        os.Exit(1)
    }
}
