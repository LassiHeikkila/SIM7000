package module

import (
	"time"
)

// Module is an interface representing the SIM7000 module
type Module interface {
	SendATCommand(cmd string, timeout time.Duration, expectedReply string) (bool, error)
	SendATCommandNoResponse(cmd string) error
	SendATCommandTwoResponses(cmd string, timeout time.Duration, expectedReply1 string, expectedReply2 string) (bool, bool, error)
	SendATCommandReturnResponse(cmd string, timeout time.Duration) ([]byte, error)
	ReadATResponse(timeout time.Duration) ([]byte, error)
	Write(buffer []byte) (int, error)
	Read(buffer []byte) (int, error)

	Close()
}

// Settings contains needed info for connecting the module to network,
// i.e. what APN to use,
// PIN for SIM card, if any,
// and which serial port to use for communicating with module
type Settings struct {
	APN                   string
	PIN                   string
	SerialPort            string
	MaxConnectionAttempts int
}
