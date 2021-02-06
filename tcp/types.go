package tcp

// CIPStatus represents module state that can be queried by AT+CIPSTATUS
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
