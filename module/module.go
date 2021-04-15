package module

import (
	"log"
	"time"
)

// Module is an interface representing the SIM7000 module
type Module interface {
	Command(cmd string) ([]string, error)
	Read(buffer []byte) (int, error)
	Write(buffer []byte) (int, error)
	RunChatScript(script ChatScript) ([]string, error)
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
	TraceLogger           *log.Logger
	ChatScript            *ChatScript
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
