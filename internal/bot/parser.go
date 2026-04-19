package bot

import "strings"

type ParsedCommand struct {
    Name string
    Arg  string
}

func ParseCommand(text string) ParsedCommand {
    text = strings.TrimSpace(text)
    if text == "" || text[0] != '/' {
        return ParsedCommand{}
    }

    parts := strings.SplitN(text, " ", 2)
    cmd := strings.ToLower(parts[0])
    arg := ""
    if len(parts) > 1 {
        arg = strings.TrimSpace(parts[1])
    }

    cmd = strings.SplitN(cmd, "@", 2)[0]
    cmd = strings.TrimPrefix(cmd, "/")

    return ParsedCommand{Name: cmd, Arg: arg}
}
