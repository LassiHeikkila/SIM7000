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