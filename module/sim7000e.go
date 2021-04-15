package module

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/warthog618/modem/at"
	"github.com/warthog618/modem/serial"
	"github.com/warthog618/modem/trace"
)

type sim7000e struct {
	modem *at.AT
	port  io.ReadWriter
	mutex sync.Mutex
}

// NewSIM7000 returns a ready to use Module
func NewSIM7000(settings Settings) Module {
	p, err := serial.New(serial.WithPort(settings.SerialPort), serial.WithBaud(115200))
	if err != nil {
		return nil
	}
	var mio io.ReadWriter
	if settings.TraceLogger != nil {
		mio = trace.New(p, trace.WithLogger(settings.TraceLogger))
	} else {
		mio = p
	}

	modem := at.New(mio, at.WithTimeout(10*time.Second))

	s := new(sim7000e)
	s.modem = modem
	s.port = mio

	s.modem.Command("+CFUN=1,1", at.WithTimeout(30*time.Second))

	s.modem.Init()

	countdown(10, time.Second)

	state := s.GetIPStatus()
	switch state {
	case IPStatus, IPClosed:
		// already setup
		print("Module already initialized!")
		return s
	}
	print("Initializing module...")
	script := defaultChatScript(settings)
	if settings.ChatScript != nil {
		script = *settings.ChatScript
	}
	_, err = s.RunChatScript(script)
	if err != nil {
		println("Initialization script failed with error:", err.Error())
		return nil
	}
	return s
}

func (s *sim7000e) Close() {
	s.Command("+CIPCLOSE")
	resp, err := s.Command("+CIPSHUT")
	_ = resp
	gotOK := false // parse resp
	if err == nil && gotOK {
		print("Connection closed successfully")
	} else {
		print("Closing connection failed")
	}
}

func constructCSTT(apn, username, password string) string {
	if username == "" && password == "" {
		return fmt.Sprintf(`+CSTT="%s"`, apn)
	}
	return fmt.Sprintf(`+CSTT="%s","%s","%s"`, apn, username, password)
}

func defaultChatScript(settings Settings) ChatScript {
	return ChatScript{
		Aborts: []string{"ERROR", "BUSY", "NO CARRIER", "+CSQ: 99,99"},
		Commands: []CommandResponse{
			NormalCommandResponse("+CSQ", "+CSQ: "),
			NormalCommandResponse("+CPIN?", "+CPIN: READY"),
			NormalCommandResponse("+CIPRXGET=1", "OK"),
			NormalCommandResponse("+CSTT?", "+CSTT: "),
			NormalCommandResponse("+CIPSTATUS", "STATE: IP INITIAL"),
			NormalCommandResponse(constructCSTT(settings.APN, settings.Username, settings.Password), "OK"),
			NormalCommandResponse("+CSTT?", fmt.Sprintf(`+CSTT: "%s"`, settings.APN)),
			NormalCommandResponse("+CIPSTATUS", "STATE: IP START"),
			CommandResponse{"+CIICR", "", 30 * time.Second, 0},
			NormalCommandResponse("+CIPSTATUS", "STATE: IP GPRSACT"),
			NormalCommandResponse("+CIFSR", ""),
			NormalCommandResponse("+CIPSTATUS", "STATE: IP STATUS"),
		},
	}
}

func (s *sim7000e) Command(cmd string) ([]string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.modem.Command(cmd)
}

func (s *sim7000e) Write(buffer []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.port.Write(buffer)
}
func (s *sim7000e) Read(buffer []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.port.Read(buffer)
}

func (s *sim7000e) RunChatScript(script ChatScript) ([]string, error) {
	containsAbortTerm := func(response []string) bool {
		for i := 0; i < len(response); i++ {
			for _, term := range script.Aborts {
				if strings.Contains(response[i], term) {
					return true
				}
			}
		}
		return false
	}
	output := make([]string, 0)
	retriesLeft := 0
	for i := range script.Commands {
		retriesLeft = script.Commands[i].Retries
	tryAtCommand:
		time.Sleep(time.Second)
		resp, err := s.modem.Command(script.Commands[i].Command, at.WithTimeout(script.Commands[i].Timeout))
		if err != nil {
			retriesLeft--
			if retriesLeft > 0 {
				goto tryAtCommand
			}
			return output, err
		}
		output = append(output, resp...)
		if containsAbortTerm(resp) {
			return output, errors.New("Reply contained abort term")
		}
		if script.Commands[i].Response == "" {
			// reply doesn't matter as long as it doesn't contain an abort term
			continue
		}
		containsResponse := func(fullResponse []string, keyword string) bool {
			for j := 0; j < len(resp); j++ {
				if strings.Contains(resp[j], keyword) {
					return true
				}
			}
			return false
		}

		if !containsResponse(resp, script.Commands[i].Response) {
			retriesLeft--
			if retriesLeft > 0 {
				goto tryAtCommand
			}
			return output, fmt.Errorf(
				"Response to \"%s\" did not contain expected \"%s\"",
				script.Commands[i].Command,
				script.Commands[i].Response,
			)
		}
	}
	return output, nil
}

func (s *sim7000e) GetIPStatus() CIPStatus {
	resp, _ := s.modem.Command("+CIPSTATUS")
	return ParseCIPSTATUSResp(resp)
}
