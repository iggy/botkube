package logs

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/morikuni/aec"

	"github.com/kubeshop/botkube/internal/cli"
)

// Printer knows how to print Botkube logs.
type Printer struct {
	podName string
	newLog  chan string
	stop    chan struct{}
	parser  JSONParser
	logger  *slog.Logger
}

// NewPrinter creates a new Printer instance.
func NewPrinter(podName string) *Printer {
	return &Printer{
		newLog: make(chan string, 10),
		stop:   make(chan struct{}),
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
		podName: podName,
		parser:  JSONParser{},
	}
}

func (f *Printer) PrintLine(line string) {
	msg, lvl, attrs, ok := f.parser.ParseLine(line)
	if !ok { // it was not recognized as JSON log entry, so let's print it as plain text.
		f.printLogLine(line)
		return
	}
	if lvl == slog.LevelDebug && !cli.VerboseMode.IsEnabled() {
		return
	}

	fmt.Print(aec.EraseLine(aec.EraseModes.Tail))
	fmt.Print(aec.Column(6))
	f.logger.LogAttrs(context.Background(), lvl, msg, attrs...)
}

func (f *Printer) printLogLine(line string) {
	fmt.Print(aec.EraseLine(aec.EraseModes.Tail))
	fmt.Print(aec.Column(6))
	fmt.Print(line)
}
