package tcp

import (
	"testing"
)

func TestDNSGIPResponseParsing(t *testing.T) {
	tests := map[string]struct {
		input     string
		wantIP1   string
		wantIP2   string
		expectErr bool
	}{
		"genericError": {
			input:     "ERROR",
			expectErr: true,
		},
		"www.example.com": {
			input: `OK

+CDNSGIP: 1,"www.example.com","93.184.216.34"`,
			wantIP1:   "93.184.216.34",
			expectErr: false,
		},
		"www.google.com": {
			input: `OK

+CDNSGIP: 1,"www.google.com","216.58.207.228"`,
			wantIP1:   "216.58.207.228",
			expectErr: false,
		},
		"not existing": {
			input: `OK

+CDNSGIP: 0,8`,
			expectErr: true,
		},
		"two ip example from docs": {
			input: `OK

+CDNSGIP: 1,"www.baidu.com","119.75.218.77","119.75.217.56"`,
			wantIP1:   "119.75.218.77",
			wantIP2:   "119.75.217.56",
			expectErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotIP1, gotIP2, err := parseDNSGIPResp([]byte(tc.input))
			if gotIP1 != tc.wantIP1 {
				t.Fatalf(`Got "%s", wanted "%s"`, gotIP1, tc.wantIP1)
			}
			if gotIP2 != tc.wantIP2 {
				t.Fatalf(`Got "%s", wanted "%s"`, gotIP2, tc.wantIP2)
			}
			if err != nil && !tc.expectErr {
				t.Fatal("Got error while not expecting one:", err)
			}
			if err == nil && tc.expectErr {
				t.Fatal("Did not get an error while expecting one!")
			}
		})
	}
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
			got := ParseCIPSTATUSResp([]byte(tc.input))
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
			got := ParseCIPSTATUSResp([]byte(tc.input))
			if got != tc.want {
				t.Fatalf(`Got %v, wanted %v`, got, tc.want)
			}
		})
	}
}
