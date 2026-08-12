package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

type Format int

const (
	FormatAuto Format = iota
	FormatJSON
	FormatPlain
)

func ResolveFormat(flagJSON, flagPlain bool) Format {
	switch {
	case flagJSON:
		return FormatJSON
	case flagPlain:
		return FormatPlain
	default:
		return FormatAuto
	}
}

func WantJSON(f Format, stdout *os.File) bool {
	switch f {
	case FormatJSON:
		return true
	case FormatPlain:
		return false
	default:
		return !term.IsTerminal(int(stdout.Fd()))
	}
}

func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func WriteError(w io.Writer, code string, message string, remediation string) error {
	return WriteJSON(w, map[string]any{
		"error":       code,
		"message":     message,
		"remediation": remediation,
	})
}

func Fprintln(w io.Writer, a ...any) {
	fmt.Fprintln(w, a...)
}
