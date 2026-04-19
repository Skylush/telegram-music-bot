package util

import "strings"

func SafeFilename(name string) string {
    cleaned := strings.Map(func(r rune) rune {
        switch r {
        case '/':
            return '／'
        case '\\':
            return '＼'
        case ':', '*', '?', '"', '<', '>', '|':
            return '_'
        default:
            return r
        }
    }, name)
    cleaned = strings.TrimSpace(cleaned)
    if cleaned == "" {
        return "song"
    }
    return cleaned
}
