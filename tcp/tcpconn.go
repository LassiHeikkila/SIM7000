// Package tcp implements tcp communications with SIM7000 module
// Currently limited to one TCP connection at a time, even though SIM7000 supports multiple connections.
package tcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"io"
	"net"
	"strings"
	"strconv"
	"time"

	"github.com/LassiHeikkila/SIM7000/module"
)

var globalSettings = map[string]string{
	`APN`:      `internet`,
	`PORT`:     `/dev/ttyS0`,
	`USERNAME`: ``,
	`PASSWORD`: ``,
	`SIMPIN`:   ``,
}

// RegisterSetting function allows the user to configure some variables in order to connect to network successfully
// These variables are:
//	"APN" - Access Point Name, default "internet"
//	"USERNAME" - username when connecting to AP, default is empty / no username. SIM7000 sets 50 character limit.
//	"PASSWORD" - password to use when connecting to AP, default is empty / no password. SIM7000 sets 50 character limit.
//  "DNS1" - primary domain name server, default is automatically received from network
//	"DNS2" - secondary domain name server, default is automatically received from network
//	"PORT" - serial port to use to talk with SIM7000X module. Default is /dev/ttyS0
//	"SIMPIN" - pin code to use for SIM card, if any is needed. Default is empty.
// RegisterSetting should be called before Dial
func RegisterSetting(key, value string) error {
	if globalSettings == nil {
		globalSettings = make(map[string]string)
	}
	globalSettings[key] = value
	return nil
}

// TCPConn implements TCP communication with SIM7000 module
// It only supports IPv4 at this time.
type TCPConn struct {
	net.Conn
	m module.Module

	localAddr  net.TCPAddr
	remoteAddr net.TCPAddr

	readDeadline  time.Time
	writeDeadline time.Time

	ctx context.Context
}

// Dial resolves the given address and opens a connection to it
// For TCP networks, the address has the form "host:port".
// The host must be a literal IP address, or a host name that can be resolved to IP addresses.
// The port must be a literal port number or a service name.
func Dial(network, addr string) (net.Conn, error) {
	return DialContext(context.Background(), network, addr)
}

// DialContext connects to the address on the named network using the provided context.
// 
// The provided Context must be non-nil. If the context expires before the connection is complete, an error is returned. Once successfully connected, any expiration of the context will not affect the connection.
// 
// When using TCP, and the host in the address parameter resolves to multiple network addresses, any dial timeout (from d.Timeout or ctx) is spread over each consecutive dial, such that each is given an appropriate fraction of the time to connect. For example, if a host has 4 IP addresses and the timeout is 1 minute, the connect to each single address will be given 15 seconds to complete before trying the next one.
// 
// See func Dial for a description of the network and address parameters
func DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
		switch network {
	case "tcp", "tcp4", "": // empty string defaults to tcp4
		return dialTCP4(ctx, addr)
	default:
		return nil, fmt.Errorf(`Unsupported network "%s"`, network)
	}
}

// GetModule returns Module ready to be used in TCP mode, provided the registered settings are OK
func GetModule() (module.Module, error) {
	s := module.Settings{
		APN:                   globalSettings[`APN`],
		Username:              globalSettings[`USERNAME`],
		Password:              globalSettings[`PASSWORD`],
		PIN:                   globalSettings[`SIMPIN`],
		SerialPort:            globalSettings[`PORT`],
		MaxConnectionAttempts: 10,
	}
	if _, ok := globalSettings[`DEBUG`]; ok {
		s.TraceLogger = log.New(log.Writer(), "MODULE TRACE:", log.Lmicroseconds)
	}
	m := module.NewSIM7000(s)
	if m == nil {
		return nil, errors.New("Failed to bring up module")
	}
	// module ready to use

	// check existing DNS config
	resp, _ := m.SendATCommandReturnResponse(`+CDNSCFG?`, time.Second)
	primary, secondary := parseDNCFGQueryResponse(resp)

	// configure DNS servers if needed / wanted
	if dns1, dns1present := globalSettings[`DNS1`]; dns1present {
		if dns2, dns2present := globalSettings[`DNS2`]; dns2present {
			if dns1 != primary || dns2 != secondary {
				if gotOK, _ := m.SendATCommand(fmt.Sprintf(`+CDNSCFG=%s,%s`, dns1, dns2), time.Second, `OK`); !gotOK {
					m.Close()
					return nil, errors.New("Failed to apply DNS configuration")
				}
			}
		} else {
			if dns1 != primary {
				if gotOK, _ := m.SendATCommand(fmt.Sprintf(`+CDNSCFG=%s`, dns1), time.Second, `OK`); !gotOK {
					m.Close()
					return nil, errors.New("Failed to apply DNS configuration")
				}
			}
		}
	}
	return m, nil
}

func dialTCP4(ctx context.Context, address string) (*TCPConn, error) {
	m, err := GetModule()
	if err != nil {
		return nil, err
	}

	var ip string
	ipOrDomain, port := parseAddress(address)
	if net.ParseIP(ipOrDomain) != nil {
		// can parse IP
		ip = ipOrDomain
	} else {
		// failed to parse IP --> must be domain name

		// resolve address
		for {
			resp, _ := m.SendATCommandReturnResponse(fmt.Sprintf(`+CDNSGIP="%s"`, ipOrDomain), 1*time.Second)
			ip1, _, err, isGarbage := parseDNSGIPResp(resp)
			if isGarbage {
				continue
			}
			if err != nil {
				fmt.Println("Failed to CDNSGIP:", ipOrDomain, err, resp)
				return nil, err
			}
			ip = ip1
			break
		}
	}

	remoteaddr := net.TCPAddr{
		IP:   net.ParseIP(ip),
		Port: port,
	}

	cipstartOK := func(resp []string) (bool, bool) {
		for _, line := range resp {
			if strings.Contains(line, "CONNECT OK") {
				return true, false
			}
			if strings.Contains(line, "ALREADY CONNECT") {
				return true, false
			}
			if strings.Contains(line, "CONNECT FAIL") {
				return false, false
			}
		}
		return false, true
	}

	for {
		resp, _ :=  m.SendATCommandReturnResponse(fmt.Sprintf(`+CIPSTART="TCP",%s,%d`, ip, port), 2*time.Second)
		if ok, isGarbage := cipstartOK(resp); isGarbage {
			continue
		} else if !ok {
			return nil, errors.New("Unable to start tcp connection")
		}
		break
	}
	fmt.Println("Connected to", ip, port)

	return &TCPConn{
		m: m,
		remoteAddr: remoteaddr,
		ctx: ctx,
	}, nil
}

func ResolveTCPAddr(network, address string) (*net.TCPAddr, error) {
	switch network {
	case "tcp", "tcp4":
		return resolveTcpAddr(network, address)
	case "":
		return resolveTcpAddr("tcp", address)
	default:
		return nil, fmt.Errorf(`Unsupported network "%s"`, network)
	}
	return nil, nil
}

func resolveTcpAddr(network, address string) (*net.TCPAddr, error) {
	return nil, nil
}

func DialTCP(network string, laddr, raddr *net.TCPAddr) (*TCPConn, error) {
	switch network {
	case "tcp", "tcp4":
	default:
		return nil, fmt.Errorf(`Bad network given: "%s"`, network)
	}
	if raddr == nil {
		return nil, errors.New(`Missing remote address`)
	}
	return dialTCP4(context.Background(), fmt.Sprintf("%s:%d", raddr.IP.String(), raddr.Port))
}

func parseBytesAvailableCIPRXGET(resp []string) (int, error) {
	for _, line := range resp {
		if strings.Contains(line, "+CIPRXGET:") {
			// we are expecting this
			// +CIPRXGET: 4,<cnflength>
			parts := strings.Split(line, `,`)
			if len(parts) < 2 {
				return 0, errors.New("Bad response: " + string(line))
			}
			cnflength := strings.TrimSpace(parts[1])
			cnflen, err := strconv.ParseInt(string(cnflength), 10, 64)
			if cnflen > 2920 {
				fmt.Println("WARNING: Module says more than 2920 bytes to be read, but max size should be 2920 according to documentation")
			}
			return int(cnflen), err
		}
	}
	return 0, errors.New("Unable to parse response")
}

func parseTCPDataCIPRXGET(resp []string, buf []byte) error {
	// response looks like this:
	// +CIPRXGET: 2,<reqlength>,<cnflength>[,<IP ADDRESS>:<PORT>]
	// 1234567890…
	// OK
	isStarted := false
	isEnded := false
	for i := 0; i < len(resp); i++ {
		if isStarted && !isEnded {
			buf = append(buf, []byte(resp[i] + "\n")...)
		}
		if isEnded {
			break
		}
		line := strings.TrimSpace(resp[i])
		if line==`OK` {
			isEnded = true
		} else if strings.Contains(line, `+CIPRXGET`) {
			isStarted = true
		}
	}

	if !isStarted || !isEnded {
		return errors.New("Incomplete response to CIPRXGET")
	}
	return nil
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
//
// Read deadline not supported yet.
func (c *TCPConn) Read(b []byte) (int, error) {
	// first ask how many unread bytes there are
	resp, _ := c.m.SendATCommandReturnResponse(`+CIPRXGET=4,1024`, time.Second)
	bytesAvail, err := parseBytesAvailableCIPRXGET(resp)
	if err != nil {
		return 0, err
	}
	if bytesAvail == 0 {
		return 0, io.EOF
	}

	resp, _ = c.m.SendATCommandReturnResponse(`+CIPRXGET=2,1024`, time.Second)
	err = parseTCPDataCIPRXGET(resp, b)
	return len(b), err
}

func checkSendOK(m module.Module, maxLines int) bool {
	scanner := bufio.NewScanner(m)
	for i := 0; i < maxLines; i++ {
		ok := scanner.Scan()
		if !ok {
			return false
		}
		resp := scanner.Text()
		s := strings.TrimSpace(resp)
		if s == `SEND OK` {
			return true
		} else if s == `SEND FAIL` {
			return false
		} else {
			fmt.Println("read:", s)
		}
	}
	return false
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (c *TCPConn) Write(b []byte) (n int, err error) {
	fmt.Println("Writing: ", string(b))
	
	parseDataSize := func(resp []string) int {
		// we are looking for this:
		// +CIPSEND: <size>
		// OK
		var sz int64
		for _, line := range resp {
			if strings.Contains(line, "+CIPSEND:") {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "+CIPSEND:")
				line = strings.TrimSpace(line)
				sz, _ = strconv.ParseInt(string(line), 10, 64)
			} else if line == "OK" {
				return int(sz)
			}
		}
		// default
		return 1460
	}
	// first check how many bytes we can send at once
	resp, _ := c.m.SendATCommandReturnResponse(`+CIPSEND?`, 100*time.Millisecond)
	fmt.Println("+CIPSEND? response:\n", resp)
	chunkSize := parseDataSize(resp)

	fmt.Printf("Writing must be done in chunks of %d bytes\n", chunkSize)
	fmt.Printf("There are %d bytes to be written\n", len(b))

	if len(b) > chunkSize {
		var tot_n = 0
		for i := 0; i < len(b); {
			if rdy, _ := c.m.SendATCommand(fmt.Sprintf(`+CIPSEND=%d`, chunkSize), time.Second, `>`); rdy {
				end := i+chunkSize
				if end > len(b) {
					end = len(b)
				}
				n, _ := c.m.Write(b[i:end])
				tot_n += n
				fmt.Printf("Wrote %d bytes, total %d/%d\n", n, tot_n, len(b))
			} else {
				fmt.Println("Module not ready to send")
				continue
			}
			success := checkSendOK(c.m, 5)
			if !success {
				fmt.Println("SEND NOK")
				return n, errors.New(`Sending failed`)
			} else {
				fmt.Println("SEND OK")
			}
			i += chunkSize
		}
		return tot_n, nil
	} else { // whole thing fits into one chunk
		if readyToSend, _ := c.m.SendATCommand(fmt.Sprintf(`+CIPSEND=%d`, len(b)), time.Second, `>`); readyToSend {
			n, err = c.m.Write(b)
			fmt.Println("Data written")
		} else {
			return 0, fmt.Errorf(`Module not ready to send`)
		}
		success := checkSendOK(c.m,5)
		if !success {
			fmt.Println("SEND NOK")
			return n, errors.New(`Sending failed`)
		}
		return n, nil
	}
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *TCPConn) Close() error {
	c.m.Close()
	return nil
}

// LocalAddr returns the local network address.
func (c *TCPConn) LocalAddr() net.Addr {
	return &c.localAddr
}

// RemoteAddr returns the remote network address.
func (c *TCPConn) RemoteAddr() net.Addr {
	return &c.remoteAddr
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (c *TCPConn) SetDeadline(t time.Time) error {
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c *TCPConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c *TCPConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}
