package module

import (
	"strings"
	"time"

	"github.com/LassiHeikkila/SIM7000/output"
)

func print(a ...interface{}) {
	output.Println(a...)
}

func printf(f string, a ...interface{}) {
	output.Printf(f, a...)
}

func dumpBytes(b []byte) string {
	builder := strings.Builder{}
	builder.Write(b)

	return builder.String()
}

func countdown(n int, interval time.Duration) {
	output.Countdown(n, interval)
}
