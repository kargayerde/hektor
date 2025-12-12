package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
)

const (
	Reset     = "\x1b[0m"
	Bold      = "\x1b[1m"
	Dim       = "\x1b[2m"
	Underline = "\x1b[4m"

	Black   = "\x1b[30m"
	Red     = "\x1b[31m"
	Green   = "\x1b[32m"
	Yellow  = "\x1b[33m"
	Blue    = "\x1b[34m"
	Magenta = "\x1b[35m"
	Cyan    = "\x1b[36m"
	White   = "\x1b[37m"

	BrightBlack   = "\x1b[90m"
	BrightRed     = "\x1b[91m"
	BrightGreen   = "\x1b[92m"
	BrightYellow  = "\x1b[93m"
	BrightBlue    = "\x1b[94m"
	BrightMagenta = "\x1b[95m"
	BrightCyan    = "\x1b[96m"
	BrightWhite   = "\x1b[97m"
)

func Colorize(s string, codes ...string) string {
	if len(codes) == 0 {
		return s
	}
	return strings.Join(codes, "") + s + Reset
}

func HTTPStatusColor(code int64) string {
	switch {
	case code >= 500:
		return Red
	case code >= 400:
		return Yellow
	default:
		return Green
	}
}

var (
	msgPrefixColors = map[string]string{
		"heartbeat": Cyan,
		"status":    Magenta,
	}
	deviceColors = map[string]string{
		"relays": Green,
		"buzzer": Yellow,
	}
)

func Setup() {
	slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: "15:04:05.000",
		AddSource:  true,
		NoColor:    false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.MessageKey {
				msg := a.Value.String()
				// Use prefix color map
				for prefix, col := range msgPrefixColors {
					if strings.HasPrefix(msg, prefix) {
						a.Value = slog.StringValue(Colorize(msg, col))
						return a
					}
				}
				return a
			}
			if a.Key == "device" {
				dev := a.Value.String()
				if col, ok := deviceColors[dev]; ok {
					a.Value = slog.StringValue(Colorize(dev, col))
				}
				return a
			}
			if len(groups) > 0 && groups[0] == "http" && a.Key == "status" {
				code := a.Value.Int64()
				a.Value = slog.StringValue(Colorize(fmt.Sprintf("%d", code), HTTPStatusColor(code)))
				return a
			}
			return a
		},
	})))
}
