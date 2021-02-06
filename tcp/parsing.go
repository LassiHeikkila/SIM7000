package tcp

import (
	"bufio"
	"errors"
	"strconv"
	"strings"
)

func parseDNSGIPResp(b []byte) (ip1 string, ip2 string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(string(b)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "+CDNSGIP:") {
			line = strings.TrimPrefix(line, "+CDNSGIP:")
			parts := strings.Split(line, ",")
			if len(parts) < 2 {
				continue
			}
			if strings.TrimSpace(parts[0]) == "0" {
				// `+GDNSGIP:0,<dns error code>`
				switch strings.TrimSpace(parts[1]) {
				case "8":
					return "", "", errors.New("Module says: DNS COMMON ERROR")
				case "3":
					return "", "", errors.New("Module says: NETWORK ERROR")
				default:
					return "", "", errors.New("Module reply could not be parsed")
				}
			}
			if strings.TrimSpace(parts[0]) == "1" {
				// `+GDNSGIP: 1,<domain name>,<IP1>[,<IP2>]`
				if len(parts) == 3 {
					// contains only IP1
					return strings.Trim(strings.TrimSpace(parts[2]), `"`), "", nil
				} else if len(parts) == 4 {
					// contains IP1 and IP2
					return strings.Trim(strings.TrimSpace(parts[2]), `"`), strings.Trim(strings.TrimSpace(parts[3]), `"`), nil
				}
			}
		}
	}
	return "", "", errors.New("No IP could be parsed from reply")
}

func ParseCIPSTATUSResp(b []byte) CIPStatus {
	scanner := bufio.NewScanner(strings.NewReader(string(b)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "STATE:") {
			state := strings.TrimSpace(strings.TrimPrefix(line, "STATE:"))
			switch state {
			case "IP INITIAL":
				return IPInitial
			case "IP START":
				return IPStart
			case "IP CONFIG":
				return IPConfig
			case "IP GPRSACT":
				return IPGPRSAct
			case "IP STATUS":
				return IPStatus
			case "TCP CONNECTING", "UDP CONNECTING", "SERVER LISTENING", "IP PROCESSING":
				return IPProcessing
			case "CONNECT OK":
				return IPConnectOK
			case "TCP CLOSING", "UDP CLOSING":
				return IPClosing
			case "TCP CLOSED", "UDP CLOSED":
				return IPClosed
			case "PDP DEACT":
				return IPPDPDeact
			default:
				return IPStatusUnknown
			}
		}
	}
	return IPStatusUnknown
}

func parseAddress(address string) (ip string, port int) {
	if strings.Contains(address, ":") {
		parts := strings.Split(address, ":")
		if len(parts) != 2 {
			return "", 0
		}
		domainOrIP := parts[0]
		portOrService := parts[1]
		if p, err := strconv.ParseInt(portOrService, 10, 64); err == nil {
			return domainOrIP, int(p)
		} else {

		}
	}
	return "", 0
}
