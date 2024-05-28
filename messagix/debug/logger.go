package debug

import (
	"encoding/hex"
	"strings"

	zerolog "github.com/rs/zerolog"
)

var colors = map[string]string{
	"text":  "\x1b[38;5;6m%s\x1b[0m",
	"debug": "\x1b[32mDEBUG\x1b[0m",
	"gray":  "\x1b[38;5;8m%s\x1b[0m",
	"info":  "\x1b[38;5;111mINFO\x1b[0m",
	"error": "\x1b[38;5;204mERROR\x1b[0m",
	"fatal": "\x1b[38;5;52mFATAL\x1b[0m",
}

func NewLogger() zerolog.Logger {
	return zerolog.DefaultContextLogger.With().Logger()
}

func BeautifyHex(data []byte) string {
	hexStr := hex.EncodeToString(data)
	result := ""
	for i := 0; i < len(hexStr); i += 2 {
		result += hexStr[i:i+2] + " "
	}

	return strings.TrimRight(result, " ")
}
