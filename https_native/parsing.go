package https

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

/* what AT commands are needed for HTTPS application?

AT+SHSSL Select SSL Configure
AT+SHCONF Set HTTP(S) Parameter
AT+SHCONN HTTP(S) Connection
AT+SHBOD Set Body
AT+SHBODEXT Set Extension Body
AT+SHAHEAD Add Head
AT+SHPARA Set HTTP(S) Para
AT+SHCPARA Clear HTTP(S) Para
AT+SHCHEAD Clear Head
AT+SHSTATE Query HTTP(S) Connection Status
AT+SHREQ Set Request Type
AT+SHREAD Read
Response Value
AT+SHDISC Disconnect HTTP(S)
AT+HTTPTOFS Download file to ap file system
AT+HTTPTOFSRL State of download file to ap file system

*/

type noImpl struct{}

func (n *noImpl) Error() string { return "Not implemented" }

func parseResponse_SHSSL_READ(r []string, idx *int, calist *string, certname *string) error {
	return parseBasicValuesEndingWithOK(r, "+SHSSL", idx, calist, certname)
}
func parseResponse_SHSSL_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

// requires special implementation
func parseResponse_SHCONF_READ(r []string, paramTag *string, paramValue *string) error {
	return &noImpl{}
}

func parseResponse_SHCONF_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHCONN(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHBOD_READ(r []string, body *string, bodyLen *int) error { return &noImpl{} }

func parseResponse_SHBOD_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHBODEXT_READ(r []string, body *string, bodyLen *int) error { return &noImpl{} }

func parseResponse_SHBODEXT_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHAHEAD_READ(r []string, typ *string, value *string) error {
	return parseBasicValuesEndingWithOK(r, "+SHAHEAD", typ, value)
}

func parseResponse_SHAHEAD_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHCHEAD(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHPARA_READ(r []string, key *string, value *string) error {
	return parseBasicValuesEndingWithOK(r, "+SHPARA", key, value)
}

func parseResponse_SHPARA_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHCPARA(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHSTATE_READ(r []string, state *int) error {
	return parseBasicValuesEndingWithOK(r, "+SHSTATE", state)
}

func parseResponse_SHREQ_READ(r []string, url *string, typ *string) error {
	return parseBasicValuesEndingWithOK(r, "+SHREQ", url, typ)
}

func parseResponse_SHREQ_UNSOLICITED_RESPONSE(r []string, typ *string, statusCode *int, length *int) error {
	return parseBasicValuesEndingWithOK(r, "+SHREQ", typ, statusCode, length)
}
func parseResponse_SHREQ_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHREAD_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}
func parseResponse_SHREAD_UNSOLICITED_RESPONSE(r []string, data *string, length *int) error {
	var readData string
	belongsToReadData := false
	rlength := 0
	for _, line := range r {
		if strings.HasPrefix(strings.TrimSpace(line), "+SHREAD:") {
			dataLenStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "+SHREAD:"))
			dataLen, _ := strconv.ParseInt(dataLenStr, 10, 64)
			rlength = int(dataLen)
			belongsToReadData = true
		} else if belongsToReadData {
			if len(readData) == 0 {
				readData = line
			} else {
				readData += "\n" + line
			}
		}
	}
	if data != nil {
		*data = readData
	}
	if length != nil {
		*length = rlength
	}
	return nil
}

func parseResponse_SHDISC(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_HTTPTOFS_READ(r []string, status *int, url *string, path *string) error {
	return &noImpl{}
}
func parseResponse_HTTPTOFS_WRITE(r []string, statusCode *int, dataLength *int) error {
	return &noImpl{}
}

func parseResponse_HTTPTOFSRL_READ(r []string) error { return &noImpl{} }

// file system commands needed
func parseResponse_CFSINIT(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_CFSTERM(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}
func parseResponse_CFSWFILE_WRITE(r []string) error {
	for _, line := range r {
		if strings.Contains(line, "DOWNLOAD") {
			return nil
		}
	}
	return errors.New("Module filesystem not ready to receive data")
}

// ssl / tls related commands
func parseResponse_CSSLCFG_WRITE(r []string) error { return &noImpl{} }
func parseResponse_CSSLCFG_WRITE_ctxindex(r []string, ctxIndex *int, sslVersion *int, cipherSuite *int, ignoreRtcTime *bool, protocol *int, sni *string) error {
	return &noImpl{}
}

func parseBasicOkOrError(r []string, ok *bool) error {
	for _, line := range r {
		if strings.Contains(line, "OK") {
			if ok != nil {
				*ok = true
			}
			return nil
		}
		if strings.Contains(line, "ERROR") {
			if ok != nil {
				*ok = false
			}
			return nil
		}
	}
	return errors.New("Reply did not contain OK or ERROR")
}

func parseBasicValuesEndingWithOK(r []string, cmd string, values ...interface{}) error {
	for _, line := range r {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, cmd+":") {
			line = strings.TrimPrefix(line, cmd+":")
			line = strings.TrimSpace(line)
			parts := strings.Split(line, ",")

			if len(parts) != len(values) {
				return fmt.Errorf("Malformed response to %s, expecting %d values, but got %d", cmd, len(values), len(parts))
			}

			for i := 0; i < len(parts); i++ {
				part := parts[i]
				val := values[i]

				if val == nil {
					// caller doesn't care about this one
					continue
				}

				switch val.(type) {
				case *string:
					*(val.(*string)) = part
				case *int:
					v, err := strconv.ParseInt(part, 10, 64)
					if err != nil {
						return err
					}
					*(val.(*int)) = int(v)
				default:
					return fmt.Errorf("Unsupported parameter type: %v", reflect.TypeOf(val))
				}
			}
		} else if strings.Contains(line, "OK") {
			return nil
		}
	}
	return fmt.Errorf("Malformed response to %s", cmd)
}
