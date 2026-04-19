package bot

import "testing"

func TestParseCommand(t *testing.T) {
    tests := []struct {
        in   string
        name string
        arg  string
    }{
        {"/search hello", "search", "hello"},
        {"/search@mybot hello world", "search", "hello world"},
        {"/help", "help", ""},
        {"hello", "", ""},
    }

    for _, tt := range tests {
        got := ParseCommand(tt.in)
        if got.Name != tt.name || got.Arg != tt.arg {
            t.Fatalf("input=%q got=%+v want name=%q arg=%q", tt.in, got, tt.name, tt.arg)
        }
    }
}
