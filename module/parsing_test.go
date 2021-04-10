package module

import (
	"testing"
	"strings"
)

func inputAsLines(input string) []string {
	return strings.Split(input, "\n")
}

func TestCIPSTATUSResponseParsingCIPMUX0(t *testing.T) {
	tests := map[string]struct {
		input string
		want  CIPStatus
	}{
		"IP INITIAL": {
			input: `OK

STATE: IP INITIAL`,
			want: IPInitial,
		},
		"IP START": {
			input: `OK

STATE: IP START`,
			want: IPStart,
		},
		"IP CONFIG": {
			input: `OK

STATE: IP CONFIG`,
			want: IPConfig,
		},
		"IP GPRSACT": {
			input: `OK

STATE: IP GPRSACT`,
			want: IPGPRSAct,
		},
		"IP STATUS": {
			input: `OK

STATE: IP STATUS`,
			want: IPStatus,
		},
		"TCP CONNECTING": {
			input: `OK

STATE: TCP CONNECTING`,
			want: IPProcessing,
		},
		"UDP CONNECTING": {
			input: `OK

STATE: UDP CONNECTING`,
			want: IPProcessing,
		},
		"SERVER LISTENING": {
			input: `OK

STATE: SERVER LISTENING`,
			want: IPProcessing,
		},
		"CONNECT OK": {
			input: `OK

STATE: CONNECT OK`,
			want: IPConnectOK,
		},
		"TCP CLOSING": {
			input: `OK

STATE: TCP CLOSING`,
			want: IPClosing,
		},
		"UDP CLOSING": {
			input: `OK

STATE: UDP CLOSING`,
			want: IPClosing,
		},
		"TCP CLOSED": {
			input: `OK

STATE: TCP CLOSED`,
			want: IPClosed,
		},
		"UDP CLOSED": {
			input: `OK

STATE: UDP CLOSED`,
			want: IPClosed,
		},
		"PDP DEACT": {
			input: `OK

STATE: PDP DEACT`,
			want: IPPDPDeact,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := ParseCIPSTATUSResp(inputAsLines(tc.input))
			if got != tc.want {
				t.Fatalf(`Got %v, wanted %v`, got, tc.want)
			}
		})
	}
}

func TestCIPSTATUSResponseParsingCIPMUX1(t *testing.T) {
	tests := map[string]struct {
		input string
		want  CIPStatus
	}{
		"IP INITIAL": {
			input: `OK
STATE: IP INITIAL
C:
0,0,"TCP","116.236.221.75","5555","CLOSED"
C:
1,1,"TCP","116.236.221.75","5555","CONNECT
ED"
C: 2,,"","","","INITIAL"
C: 3,,"","","","INITIAL"
C: 4,,"","","","INITIAL"
C: 5,,"","","","INITIAL"
C: 6,,"","","","INITIAL"
C: 7,,"","","","INITIAL"`,
			want: IPInitial,
		},
		"IP START": {
			input: `OK
STATE: IP START
C:
0,0,"TCP","116.236.221.75","5555","CLOSED"
C:
1,1,"TCP","116.236.221.75","5555","CONNECT
ED"
C: 2,,"","","","INITIAL"
C: 3,,"","","","INITIAL"
C: 4,,"","","","INITIAL"
C: 5,,"","","","INITIAL"
C: 6,,"","","","INITIAL"
C: 7,,"","","","INITIAL"`,
			want: IPStart,
		},
		"IP CONFIG": {
			input: `OK
STATE: IP CONFIG
C:
0,0,"TCP","116.236.221.75","5555","CLOSED"
C:
1,1,"TCP","116.236.221.75","5555","CONNECT
ED"
C: 2,,"","","","INITIAL"
C: 3,,"","","","INITIAL"
C: 4,,"","","","INITIAL"
C: 5,,"","","","INITIAL"
C: 6,,"","","","INITIAL"
C: 7,,"","","","INITIAL"`,
			want: IPConfig,
		},
		"IP GPRSACT": {
			input: `OK
STATE: IP GPRSACT
C:
0,0,"TCP","116.236.221.75","5555","CLOSED"
C:
1,1,"TCP","116.236.221.75","5555","CONNECT
ED"
C: 2,,"","","","INITIAL"
C: 3,,"","","","INITIAL"
C: 4,,"","","","INITIAL"
C: 5,,"","","","INITIAL"
C: 6,,"","","","INITIAL"
C: 7,,"","","","INITIAL"`,
			want: IPGPRSAct,
		},
		"IP STATUS": {
			input: `OK
STATE: IP STATUS
C:
0,0,"TCP","116.236.221.75","5555","CLOSED"
C:
1,1,"TCP","116.236.221.75","5555","CONNECT
ED"
C: 2,,"","","","INITIAL"
C: 3,,"","","","INITIAL"
C: 4,,"","","","INITIAL"
C: 5,,"","","","INITIAL"
C: 6,,"","","","INITIAL"
C: 7,,"","","","INITIAL"`,
			want: IPStatus,
		},
		"IP PROCESSING": {
			input: `OK
STATE: IP PROCESSING
C:
0,0,"TCP","116.236.221.75","5555","CLOSED"
C:
1,1,"TCP","116.236.221.75","5555","CONNECT
ED"
C: 2,,"","","","INITIAL"
C: 3,,"","","","INITIAL"
C: 4,,"","","","INITIAL"
C: 5,,"","","","INITIAL"
C: 6,,"","","","INITIAL"
C: 7,,"","","","INITIAL"`,
			want: IPProcessing,
		},
		"PDP DEACT": {
			input: `OK
STATE: PDP DEACT
C:
0,0,"TCP","116.236.221.75","5555","CLOSED"
C:
1,1,"TCP","116.236.221.75","5555","CONNECT
ED"
C: 2,,"","","","INITIAL"
C: 3,,"","","","INITIAL"
C: 4,,"","","","INITIAL"
C: 5,,"","","","INITIAL"
C: 6,,"","","","INITIAL"
C: 7,,"","","","INITIAL"`,
			want: IPPDPDeact,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := ParseCIPSTATUSResp(inputAsLines(tc.input))
			if got != tc.want {
				t.Fatalf(`Got %v, wanted %v`, got, tc.want)
			}
		})
	}
}
