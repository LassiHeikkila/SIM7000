package tcp

import (
	"bufio"
	"bytes"
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

func parseDNCFGQueryResponse(resp []byte) (primary string, secondary string) {
	lines := bytes.Split(resp, []byte("\n"))

	extractIP := func(line []byte) []byte {
		parts := bytes.Split(line, []byte(`:`))
		if len(parts) != 2 {
			return nil
		}
		return bytes.TrimSpace(parts[1])
	}

	for _, line := range lines {
		if bytes.HasPrefix(line, []byte(`PrimaryDns:`)) {
			ip := extractIP(line)
			if ip != nil {
				primary = string(ip)
			}
		} else if bytes.HasPrefix(line, []byte(`SecondaryDns:`)) {
			ip := extractIP(line)
			if ip != nil {
				secondary = string(ip)
			}
		}
	}
	return
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
		}
		// can't parse port as int, must be a string
		if p_num, ok := parseService(portOrService); ok {
			return domainOrIP, int(p_num)
		}
	}
	return "", 0
}

// convert service to port number
// only for well known basic services
// e.g. 
// 	SSH 	-> 22
// 	HTTP 	-> 80
// 	HTTPS 	-> 443
// returns port number and boolean indicating if service was resolved
// if known service, returns default port and true
// if unknown service, returns 0 and false
func parseService(service string) (uint16, bool) {
	switch service {
	case "ssh":
		return 22, true
	case "telnet":
		return 23, true
	case "smtp":
		return 25, true
	case "dns":
		return 53, true
	case "http":
		return 80, true
	case "https":
		return 443, true
	case "ntp":
		return 123, true
	default:
		return 0, false
	}
}