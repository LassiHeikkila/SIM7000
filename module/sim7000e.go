package module

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tarm/serial"
)

type sim7000e struct {
	port *serial.Port
}

// NewSIM7000 returns a ready to use Module
func NewSIM7000(settings Settings) Module {
	c := serial.Config{
		Name:        settings.SerialPort,
		Baud:        115200,
		Size:        8,
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
		ReadTimeout: time.Second,
	}

	p, err := serial.OpenPort(&c)
	if err != nil {
		return nil
	}

	constructCSTT := func(apn, username, password string) string {
		if len(apn) > 50 {
			panic("APN too long, maximum length is 50")
		}
		if len(username) > 50 {
			panic("USERNAME too long, maximum length is 50")
		}
		if len(password) > 50 {
			panic("PASSWORD too long, maximum length is 50")
		}
		if username == "" && password == "" {
			return fmt.Sprintf(`AT+CSTT="%s"`, settings.APN)
		}
		return fmt.Sprintf(`AT+CSTT="%s","%s","%s"`, settings.APN, settings.Username, settings.Password)
	}

	s := new(sim7000e)
	s.port = p
	state := s.GetIPStatus()
	switch state {
	case IPStatus, IPClosed:
		// already setup
		return s
	}
	print("Initializing module...")
	script := ChatScript{
		Aborts: []string{"ERROR", "BUSY", "NO CARRIER", "+CSQ: 99,99"},
		Commands: []CommandResponse{
			CommandResponse{"AT", "OK", time.Second, 10},
			CommandResponse{"AT+CFUN=1,1", "OK", 10 * time.Second, 0},
			CommandResponse{"AT", "OK", time.Second, 10},
			NormalCommandResponse("ATE0", "OK"),
			NormalCommandResponse("AT+CSQ", "+CSQ: "),
			NormalCommandResponse("AT+CPIN?", "+CPIN: READY"),
			NormalCommandResponse("AT+CSTT?", "+CSTT: "),
			NormalCommandResponse("AT+CIPSTATUS", "STATE: IP INITIAL"),
			NormalCommandResponse(constructCSTT(settings.APN, settings.Username, settings.Password), "OK"),
			NormalCommandResponse("AT+CSTT?", fmt.Sprintf(`+CSTT: "%s"`, settings.APN)),
			NormalCommandResponse("AT+CIPSTATUS", "STATE: IP START"),
			CommandResponse{"AT+CIICR", "OK", 30 * time.Second, 0},
			NormalCommandResponse("AT+CIPSTATUS", "STATE: IP GPRSACT"),
			NormalCommandResponse("AT+CIFSR", ""),
			NormalCommandResponse("AT+CIPSTATUS", "STATE: IP STATUS"),
		},
	}
	output, err := s.RunChatScript(script)
	if err != nil {
		println("Initialization script failed with error:", err.Error())
		println("Output from script:\n", dumpBytes(output))
		return nil
	}
	println("Output from script:\n", dumpBytes(output))

	return s
}

func (s *sim7000e) Close() {
	s.SendATCommandTwoResponses("AT+CIPCLOSE", 5*time.Second, "OK", "ERROR")
	if gotOK, err := s.SendATCommand("AT+CIPSHUT", 5*time.Second, "OK"); err == nil && gotOK {
		print("Connection closed successfully")
	} else {
		print("Closing connection failed")
	}
	s.port.Close()
}

func (s *sim7000e) SendATCommand(cmd string, timeout time.Duration, expectedReply string) (bool, error) {
	s.port.Flush()
	_, err := s.port.Write([]byte(cmd + "\r"))
	if err != nil {
		return false, fmt.Errorf("Failed to write command: %s", err)
	}

	response, _ := s.ReadATResponse(timeout)

	print("Got response:", dumpBytes(response))

	if bytes.Contains(response, []byte(expectedReply)) {
		return true, nil
	}

	return false, errors.New("Did not get expected reply")
}

func (s *sim7000e) SendATCommandReturnResponse(cmd string, timeout time.Duration) ([]byte, error) {
	s.port.Flush()
	s.port.Write([]byte(cmd + "\r\n"))

	return s.ReadATResponse(timeout)
}

func (s *sim7000e) SendATCommandNoResponse(cmd string) error {
	s.port.Flush()
	s.port.Write([]byte(cmd + "\r"))
	return nil
}
func (s *sim7000e) SendATCommandTwoResponses(cmd string, timeout time.Duration, expectedReply1 string, expectedReply2 string) (bool, bool, error) {
	s.port.Flush()
	_, err := s.port.Write([]byte(cmd + "\r"))
	if err != nil {
		return false, false, fmt.Errorf("Failed to write command: %s", err)
	}

	response, _ := s.ReadATResponse(timeout)
	print("Got response:", dumpBytes(response))

	if bytes.Contains(response, []byte(expectedReply1)) {
		return true, false, nil
	}
	if bytes.Contains(response, []byte(expectedReply2)) {
		return false, true, nil
	}
	return false, false, errors.New("Did not get expected reply")
}

func (s *sim7000e) Write(buffer []byte) (int, error) {
	return s.port.Write(buffer)
}
func (s *sim7000e) Read(buffer []byte) (int, error) {
	return s.port.Read(buffer)
}
func (s *sim7000e) ReadFull(buffer []byte) (int, error) {
	totalN := 0
	for len(buffer) < cap(buffer) {
		n, err := s.port.Read(buffer[len(buffer):])
		totalN += n
		if err != nil {
			return totalN, err
		}
	}

	return totalN, nil
}
func (s *sim7000e) ReadUntilNull() ([]byte, error) {
	return s.ReadUntilDelim(0x0)
}
func (s *sim7000e) ReadUntilDelim(delim byte) ([]byte, error) {
	reader := bufio.NewReader(s.port)
	b, err := reader.ReadBytes(delim)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *sim7000e) ReadATResponse(timeout time.Duration) ([]byte, error) {
	// AT command response starts and ends with <CR><LF>
	// so e.g.:
	//     <CR><LF>OK<CR><LF>
	//
	// So, we should read until we've seen two <CR><LF> and output the stuff in between

	reader := bufio.NewReader(s.port)
	resp := make([][]byte, 0)

	start := time.Now()

	for {
		elapsed := time.Now().Sub(start)
		if elapsed > timeout {
			break
		}
		//print("reading")
		line, _, err := reader.ReadLine()
		if err != nil {
			continue
		}
		print(">>", dumpBytes(line))
		c := make([]byte, len(line))
		copy(c, line)
		resp = append(resp, c)
	}
	return bytes.Join(resp, []byte("\n")), nil
}

func (s *sim7000e) RunChatScript(script ChatScript) ([]byte, error) {
	containsAbortTerm := func(response []byte) bool {
		for _, term := range script.Aborts {
			if strings.Contains(string(response), term) {
				return true
			}
		}
		return false
	}
	output := make([]byte, 0, 64)
	retriesLeft := 0
	for i := range script.Commands {
		retriesLeft = script.Commands[i].Retries
	tryAtCommand:
		resp, err := s.SendATCommandReturnResponse(script.Commands[i].Command, script.Commands[i].Timeout)
		if err != nil {
			return output, err
		}
		output = append(output, resp...)
		if containsAbortTerm(resp) {
			return output, fmt.Errorf("Aborted with %s", string(resp))
		}
		if script.Commands[i].Response == "" {
			// reply doesn't matter as long as it doesn't contain an abort term
			continue
		}
		if !strings.Contains(string(resp), script.Commands[i].Response) {
			retriesLeft--
			if retriesLeft > 0 {
				goto tryAtCommand
			}
			return output, fmt.Errorf(
				"Response \"%s\" did not contain expected \"%s\"",
				string(resp),
				script.Commands[i].Response,
			)
		}
	}
	return output, nil
}

func (s *sim7000e) GetIPStatus() CIPStatus {
	resp, _ := s.SendATCommandReturnResponse(`AT+CIPSTATUS`, time.Second)
	return ParseCIPSTATUSResp(resp)
}