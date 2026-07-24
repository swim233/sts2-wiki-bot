package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	defaultModule = "sts2bot"
	timeLayout    = "2006/01/02 - 15:04:05"
	ansiReset     = "\x1b[0m"
	ansiCyan      = "\x1b[36m"
	ansiGreen     = "\x1b[32m"
	ansiYellow    = "\x1b[33m"
	ansiRed       = "\x1b[31m"
)

type ColorMode uint8

const (
	ColorAuto ColorMode = iota
	ColorAlways
	ColorNever
)

type Options struct {
	Level slog.Leveler
	Color ColorMode
}

type attrBlock struct {
	groups []string
	attrs  []slog.Attr
}

type Handler struct {
	out          io.Writer
	level        slog.Leveler
	colorEnabled bool
	mu           *sync.Mutex
	groups       []string
	attrs        []attrBlock
}

func NewHandler(out io.Writer, options Options) *Handler {
	if out == nil {
		out = io.Discard
	}
	return &Handler{
		out:          out,
		level:        options.Level,
		colorEnabled: resolveColor(out, options.Color),
		mu:           &sync.Mutex{},
	}
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	minimum := slog.LevelInfo
	if h.level != nil {
		minimum = h.level.Level()
	}
	return level >= minimum
}

func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	if !h.Enabled(ctx, record.Level) {
		return nil
	}

	module, attrs := h.collectAttrs(record)
	var line bytes.Buffer
	fmt.Fprintf(&line, "[%s] %s  ", safeModule(module), record.Time.Local().Format(timeLayout))
	writeLevel(&line, record.Level, h.colorEnabled)
	fmt.Fprintf(&line, "  %s  %s", sourceLocation(record), safeMessage(record.Message))
	for _, attr := range attrs {
		fmt.Fprintf(&line, " %s=%s", attr.key, strconv.Quote(attr.value))
	}
	line.WriteByte('\n')

	h.mu.Lock()
	n, err := h.out.Write(line.Bytes())
	h.mu.Unlock()
	if err != nil {
		return err
	}
	if n != line.Len() {
		return io.ErrShortWrite
	}
	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.attrs = append([]attrBlock(nil), h.attrs...)
	clone.attrs = append(clone.attrs, attrBlock{
		groups: append([]string(nil), h.groups...),
		attrs:  append([]slog.Attr(nil), attrs...),
	})
	return &clone
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append([]string(nil), h.groups...)
	clone.groups = append(clone.groups, name)
	return &clone
}

type renderedAttr struct {
	key   string
	value string
}

func (h *Handler) collectAttrs(record slog.Record) (string, []renderedAttr) {
	module := defaultModule
	var rendered []renderedAttr
	for _, block := range h.attrs {
		for _, attr := range block.attrs {
			appendAttr(&rendered, &module, block.groups, attr)
		}
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendAttr(&rendered, &module, h.groups, attr)
		return true
	})
	return module, rendered
}

func appendAttr(rendered *[]renderedAttr, module *string, groups []string, attr slog.Attr) {
	if attr.Equal(slog.Attr{}) {
		return
	}
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		nestedGroups := groups
		if attr.Key != "" {
			nestedGroups = append(append([]string(nil), groups...), attr.Key)
		}
		for _, child := range value.Group() {
			appendAttr(rendered, module, nestedGroups, child)
		}
		return
	}
	if len(groups) == 0 && attr.Key == "module" && value.Kind() == slog.KindString {
		if candidate := strings.TrimSpace(value.String()); candidate != "" {
			*module = candidate
		}
		return
	}
	if attr.Key == "" {
		return
	}
	parts := append(append([]string(nil), groups...), attr.Key)
	for i := range parts {
		parts[i] = safeKey(parts[i])
	}
	*rendered = append(*rendered, renderedAttr{
		key:   strings.Join(parts, "."),
		value: valueText(value),
	})
}

func valueText(value slog.Value) string {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindBool:
		return strconv.FormatBool(value.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(value.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(value.Float64(), 'g', -1, 64)
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format("2006-01-02T15:04:05.000Z07:00")
	case slog.KindAny:
		if err, ok := value.Any().(error); ok {
			return err.Error()
		}
		return fmt.Sprint(value.Any())
	default:
		return value.String()
	}
}

func writeLevel(buffer *bytes.Buffer, level slog.Level, color bool) {
	label := strings.ToUpper(level.String())
	if color {
		buffer.WriteString(levelColor(level))
		buffer.WriteString(label)
		buffer.WriteString(ansiReset)
	} else {
		buffer.WriteString(label)
	}
	if padding := 5 - utf8.RuneCountInString(label); padding > 0 {
		buffer.WriteString(strings.Repeat(" ", padding))
	}
}

func levelColor(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return ansiCyan
	case level < slog.LevelWarn:
		return ansiGreen
	case level < slog.LevelError:
		return ansiYellow
	default:
		return ansiRed
	}
}

func sourceLocation(record slog.Record) string {
	if record.PC == 0 {
		return "unknown:0"
	}
	source := record.Source()
	return fmt.Sprintf("%s:%d", shortenPath(source.File), source.Line)
}

func shortenPath(path string) string {
	path = filepath.ToSlash(path)
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return "unknown"
	}
	if len(parts) == 1 || (len(parts) >= 2 && parts[len(parts)-2] == "sts2bot") {
		return parts[len(parts)-1]
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func safeMessage(message string) string {
	if message == "" || !utf8.ValidString(message) {
		return strconv.Quote(message)
	}
	for _, r := range message {
		if unicode.IsControl(r) || r == '\x1b' {
			return strconv.Quote(message)
		}
	}
	return message
}

func safeModule(module string) string {
	if module == "" || needsQuote(module) || strings.ContainsAny(module, "[]") {
		return strconv.Quote(module)
	}
	return module
}

func safeKey(key string) string {
	if key == "" || needsQuote(key) || strings.ContainsAny(key, ".[]") {
		return strconv.Quote(key)
	}
	return key
}

func needsQuote(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return true
	}
	for _, r := range value {
		if unicode.IsControl(r) || r == '\x1b' || r == '"' || r == '=' || unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func resolveColor(out io.Writer, mode ColorMode) bool {
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	}
	if noColor, ok := os.LookupEnv("NO_COLOR"); ok && noColor != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeDevice != 0 && info.Mode()&os.ModeCharDevice != 0
}
