package module

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/tarm/serial"
)

type sim7000e struct {
	port *serial.Port
}

// NewSIM7000E returns a ready to use Module
func NewSIM7000E(settings Settings) Module {
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

	s := new(sim7000e)
	s.port = p
	retries := 0
	print("Trying to connect to module...")
	for {
		if gotOK, _ := s.SendATCommand("AT", time.Second, "OK"); gotOK {
			break
		}
		time.Sleep(time.Second)
		retries++
		if retries > settings.MaxConnectionAttempts {
			return nil
		}
		print("Retry in 1 second...")
	}
	success := new(bool)
	defer func() {
		if !*success {
			s.Close()
		}
	}()

	print("Resetting module")
	s.SendATCommand("AT+CFUN=1,1", 10*time.Second, "OK")
	countdown(15, time.Second)

	print("Trying to connect to module...")
	for {
		if gotOK, _ := s.SendATCommand("AT", time.Second, "OK"); gotOK {
			break
		}
		time.Sleep(time.Second)
		retries++
		if retries > settings.MaxConnectionAttempts {
			return nil
		}
		print("Retry in 1 second...")
	}

	print("Setting echo mode: off")
	s.SendATCommandNoResponse("ATE0")

	print("Getting signal quality...")
	response, _ := s.SendATCommandReturnResponse("AT+CSQ", 2*time.Second)
	if bytes.Contains(response, []byte("ERROR")) {
		print("Error getting signal quality report")
		return nil
	} else {
		print("Got signal quality:", dumpBytes(response))
	}

	countdown(5, time.Second)

	print("Checking supported APNs")
	response, _ = s.SendATCommandReturnResponse("AT+CGNAPN", 2*time.Second)
	print("Response:", dumpBytes(response))

	print("Checking current APN setting")
	response, _ = s.SendATCommandReturnResponse("AT+CSTT?", 2*time.Second)
	print("Response:", dumpBytes(response))
	if bytes.Contains(response, []byte(fmt.Sprintf("\"%s\"", settings.APN))) {
		print("APN is already correct")
	} else {
		printf("Setting APN to \"%s\"\n", settings.APN)
		if gotOK, gotError, _ := s.SendATCommandTwoResponses(fmt.Sprintf("AT+CSTT=\"%s\"", settings.APN), 2*time.Second, "OK", "ERROR"); gotOK {
			print("APN set successfully")
		} else if gotError {
			print("Setting APN failed")
			return nil
		}
	}

	countdown(5, time.Second)

	print("Checking APN is correct")
	s.SendATCommandNoResponse("AT+CSTT?")
	response, _ = s.ReadATResponse(2 * time.Second)
	if bytes.Contains(response, []byte(settings.APN)) {
		print("APN is correct")
	} else {
		print("APN is wrong, response: ", dumpBytes(response))
		return nil
	}

	print("Bringing up connection")
	if gotOK, gotError, _ := s.SendATCommandTwoResponses("AT+CIICR", 2*time.Second, "OK", "ERROR"); gotOK {
		print("Connection brought up successfully")
	} else if gotError {
		print("Failed to bring up connection")
		return nil
	}

	countdown(5, time.Second)

	print("Getting IP address")
	if gotError, _ := s.SendATCommand("AT+CIFSR", 2*time.Second, "ERROR"); !gotError {
		b, _ := s.ReadATResponse(time.Second)
		print("Got IP address:", dumpBytes(b))
	} else if gotError {
		print("Error checking IP address")
		return nil
	}

	print("Checking current state")
	response, _ = s.SendATCommandReturnResponse("AT+CIPSTATUS", 5*time.Second)
	print("response:", dumpBytes(response))

	*success = true
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
	s.port.Write([]byte(cmd + "\r"))

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
	resp := make([][]byte, 1)

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
