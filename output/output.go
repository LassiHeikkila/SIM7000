package output

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/gosuri/uiprogress"
)

var outputWriter io.Writer

func init() {
	outputWriter = ioutil.Discard
}

// SetWriter allows the consumer of this package to
// choose where this package writes output.
//
// Default is to discard all output
func SetWriter(w io.Writer) {
	outputWriter = w
}

// Print calls fmt.Fprint() with the configured writer
func Print(a ...interface{}) {
	fmt.Fprint(outputWriter, a...)
}

// Println calls fmt.Fprintln() with the configured writer
func Println(a ...interface{}) {
	fmt.Fprintln(outputWriter, a...)
}

// Printf calls fmt.Fprintf() with the configured writer
func Printf(f string, a ...interface{}) {
	fmt.Fprintf(outputWriter, f, a...)
}

func Countdown(n int, interval time.Duration) {
	progress := uiprogress.New()
	progress.SetOut(outputWriter)
	progress.Start()
	bar := progress.AddBar(n)
	bar.PrependElapsed()
	bar.AppendCompleted()

	for i := 0; i <= n; i++ {
		bar.Incr()
		time.Sleep(interval)
	}

	progress.Stop()
}
