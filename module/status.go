package module

import (
	"strings"
)

// CIPStatus represents module state that can be queried by +CIPSTATUS
type CIPStatus int8

// Possible states defined on page 152 of SIM7000 Series AT Command Manual V1.06
const (
	IPStatusUnknown CIPStatus = iota
	IPInitial
	IPStart
	IPConfig
	IPGPRSAct
	IPStatus
	IPProcessing
	IPConnectOK
	IPClosing
	IPClosed
	IPPDPDeact
)

func ParseCIPSTATUSResp(resp []string) CIPStatus {
	for i := 0; i < len(resp); i++ {
		line := strings.TrimSpace(resp[i])
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
