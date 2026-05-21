package logs

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/kubeshop/botkube/internal/cli"
)

// JSONParser knows how to parse JSON formatted logs.
type JSONParser struct{}

// ParseLine returns the parsed log line as a message, level, and additional
// structured attributes suitable for use with log/slog.
func (j *JSONParser) ParseLine(line string) (msg string, lvl slog.Level, attrs []slog.Attr, ok bool) {
	result := j.parseLine(line)
	if result == nil {
		return "", 0, nil, false
	}

	lvl = parseLevel(fmt.Sprint(result["level"]))
	if m, mOk := result["msg"]; mOk {
		msg = fmt.Sprint(m)
	}

	keys := maps.Keys(result)
	sort.Strings(keys)
	for _, k := range keys {
		switch k {
		case "level", "msg", "time": // already processed
			continue
		case "component", "url":
			if !cli.VerboseMode.IsEnabled() {
				continue // ignore those fields if verbose is not enabled
			}
		}
		attrs = append(attrs, slog.Any(k, result[k]))
	}

	return msg, lvl, attrs, true
}

func (*JSONParser) parseLine(line string) map[string]any {
	var out map[string]any
	err := json.Unmarshal([]byte(line), &out)
	if err != nil {
		return nil
	}
	return out
}

// parseLevel takes a string level and returns the corresponding slog.Level.
// Panic and fatal are mapped to slog.LevelError + 4 to preserve their
// higher-than-error severity ordering.
func parseLevel(lvl string) slog.Level {
	switch strings.ToLower(lvl) {
	case "panic", "fatal":
		return slog.LevelError + 4
	case "error", "err":
		return slog.LevelError
	case "warn", "warning":
		return slog.LevelWarn
	case "info":
		return slog.LevelInfo
	case "debug", "trace":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}
