package tcp

import (
	"errors"
	"fmt"
	"net"
	"strings"
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
}

// Dial resolves the given address and opens a connection to it
// For TCP networks, the address has the form "host:port".
// The host must be a literal IP address, or a host name that can be resolved to IP addresses.
// The port must be a literal port number or a service name.
func Dial(network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "": // empty string defaults to tcp4
		return dialTCP4(address)
	default:
		return nil, fmt.Errorf(`Unsupported network "%s"`, network)
	}
}

func getModule() (module.Module, error) {
	s := module.Settings{
		APN:                   globalSettings[`APN`],
		Username:              globalSettings[`USERNAME`],
		Password:              globalSettings[`PASSWORD`],
		PIN:                   globalSettings[`SIMPIN`],
		SerialPort:            globalSettings[`PORT`],
		MaxConnectionAttempts: 10,
	}
	m := module.NewSIM7000(s)
	if m == nil {
		return nil, errors.New("Failed to bring up module")
	}
	// module ready to use

	// check existing DNS config
	resp, _ := m.SendATCommandReturnResponse(`AT+CDNSCFG?`, time.Second)
	primary, secondary := parseDNCFGQueryResponse(resp)

	// configure DNS servers if needed / wanted
	if dns1, dns1present := globalSettings[`DNS1`]; dns1present {
		if dns2, dns2present := globalSettings[`DNS2`]; dns2present {
			if dns1 != primary || dns2 != secondary {
				if gotOK, _ := m.SendATCommand(fmt.Sprintf(`AT+CDNSCFG=%s,%s`, dns1, dns2), time.Second, `OK`); !gotOK {
					m.Close()
					return nil, errors.New("Failed to apply DNS configuration")
				}
			}
		} else {
			if dns1 != primary {
				if gotOK, _ := m.SendATCommand(fmt.Sprintf(`AT+CDNSCFG=%s`, dns1), time.Second, `OK`); !gotOK {
					m.Close()
					return nil, errors.New("Failed to apply DNS configuration")
				}
			}
		}
	}
	return m, nil
}

func dialTCP4(address string) (*TCPConn, error) {
	m, err := getModule()
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
		resp, _ := m.SendATCommandReturnResponse(fmt.Sprintf(`AT+CDNSGIP="%s"`, address), 1*time.Second)
		ip1, _, err := parseDNSGIPResp(resp)
		if err != nil {
			return nil, err
		}
		ip = ip1
	}

	_ = net.TCPAddr{
		IP:   net.ParseIP(ip),
		Port: port,
	}

	_, _=  m.SendATCommandReturnResponse(fmt.Sprintf(`AT+CIPSTART="TCP",%s,%d`, ip, port), 2*time.Second)

	return nil, nil
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
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf(`Bad network given: "%s"`, network)
	}
	if raddr == nil {
		return nil, errors.New(`Missing remote address`)
	}
	return nil, nil
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
//
// Read deadline not supported yet.
func (c *TCPConn) Read(b []byte) (n int, err error) {
	//d := time.Until(c.readDeadline)
	n, err = c.m.Read(b)
	return n, err
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (c *TCPConn) Write(b []byte) (n int, err error) {
	d := time.Second
	if !c.writeDeadline.IsZero() {
		d = time.Until(c.writeDeadline)
	}
	if readyToSend, _ := c.m.SendATCommand(fmt.Sprintf(`AT+CIPSEND=%d`, len(b)), d, `>`); readyToSend {
		n, err = c.m.Write(b)
	} else {
		return 0, fmt.Errorf(`Module not ready to send`)
	}
	d = time.Second
	if !c.writeDeadline.IsZero() {
		d = time.Until(c.writeDeadline)
	}
	resp, _ := c.m.ReadATResponse(d)
	s := strings.TrimSpace(string(resp))
	if s != `SEND OK` {
		return n, fmt.Errorf(`Sending failed, got module response: %s`, s)
	}

	return n, nil
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
