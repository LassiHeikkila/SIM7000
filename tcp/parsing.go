package tcp

import (
	"errors"
	"strconv"
	"strings"
)

func parseDNSGIPResp(resp []string) (ip1 string, ip2 string, err error, isGarbage bool) {
	for i := 0; i < len(resp); i++ {
		line := strings.TrimSpace(resp[i])
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
					return "", "", errors.New("Module says: DNS COMMON ERROR"), false
				case "3":
					return "", "", errors.New("Module says: NETWORK ERROR"), false
				default:
					return "", "", errors.New("Module reply could not be parsed"), false
				}
			}
			if strings.TrimSpace(parts[0]) == "1" {
				// `+GDNSGIP: 1,<domain name>,<IP1>[,<IP2>]`
				if len(parts) == 3 {
					// contains only IP1
					return strings.Trim(strings.TrimSpace(parts[2]), `"`), "", nil, false
				} else if len(parts) == 4 {
					// contains IP1 and IP2
					return strings.Trim(strings.TrimSpace(parts[2]), `"`), strings.Trim(strings.TrimSpace(parts[3]), `"`), nil, false
				}
			}
		}
	}
	return "", "", errors.New("No IP could be parsed from reply"), true
}

func parseDNCFGQueryResponse(resp []string) (primary string, secondary string) {
	extractIP := func(line string) string {
		parts := strings.Split(line, `:`)
		if len(parts) != 2 {
			return ""
		}
		return strings.TrimSpace(parts[1])
	}

	for _, line := range resp {
		if strings.HasPrefix(line, `PrimaryDns:`) {
			ip := extractIP(line)
			if ip != "" {
				primary = ip
			}
		} else if strings.HasPrefix(line, `SecondaryDns:`) {
			ip := extractIP(line)
			if ip != "" {
				secondary = ip
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