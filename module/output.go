package module

import (
	"fmt"
	"strings"
	"time"
	"unicode"

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
	for _, char := range b {
		if unicode.IsPrint(rune(char)) {
			builder.WriteRune(rune(char))
		} else {
			builder.WriteString(fmt.Sprintf("[0x%X]", char))
		}
	}

	return builder.String()
}

func countdown(n int, interval time.Duration) {
	output.Countdown(n, interval)
}
