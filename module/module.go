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
	RunChatScript(script ChatScript) ([]byte, error)
	GetIPStatus() CIPStatus 

	Close()
}

// Settings contains needed info for connecting the module to network,
// i.e. what APN to use, username and password for APN,
// PIN for SIM card, if any (not supported yet),
// and which serial port to use for communicating with module
type Settings struct {
	APN                   string
	Username              string
	Password              string
	PIN                   string
	SerialPort            string
	MaxConnectionAttempts int
}

type ChatScript struct {
	Aborts   []string
	Commands []CommandResponse
}

type CommandResponse struct {
	Command  string
	Response string
	Timeout  time.Duration
	Retries  int
}

func NormalCommandResponse(cmd string, resp string) CommandResponse {
	return CommandResponse{cmd, resp, 100 * time.Millisecond, 0}
}
