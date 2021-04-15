package https

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	//"github.com/LassiHeikkila/SIM7000/output"
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
func parseResponse_SHCONF_READ(
	r []string,
	url *string,
	timeout *int,
	bodylen *int,
	headerlen *int,
	pollcnt *int,
	pollintms *int,
	ipver *int,
) error {
	shouldParse := false
	for _, line := range r {
		if strings.TrimSpace(line) == "OK" {
			return nil
		}
		if strings.TrimSpace(line) == "+SHCONF:" {
			shouldParse = true
			continue
		}
		if !shouldParse {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			// this isn't something we expected
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "URL":
			if url != nil {
				*url = value
			}
		case "TIMEOUT":
			if timeout != nil {
				v, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					continue
				}
				*timeout = int(v)
			}
		case "BODYLEN":
			if bodylen != nil {
				v, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					continue
				}
				*bodylen = int(v)
			}
		case "HEADERLEN":
			if headerlen != nil {
				v, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					continue
				}
				*headerlen = int(v)
			}
		case "POLLCNT":
			if pollcnt != nil {
				v, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					continue
				}
				*pollcnt = int(v)
			}
		case "POLLINTMS":
			if pollintms != nil {
				v, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					continue
				}
				*pollintms = int(v)
			}
		case "IPVER":
			if ipver != nil {
				v, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					continue
				}
				*ipver = int(v)
			}
		}
	}
	return errors.New("Unexpected end of response")
}

func parseResponse_SHCONF_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHCONN(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseResponse_SHBOD_READ(r []string, body *string, bodyLen *int) error {
	return parseBasicValuesEndingWithOK(r, "+SHBOD", body, bodyLen)
}

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
func parseResponse_CSSLCFG_WRITE(r []string, ok *bool) error {
	return parseBasicOkOrError(r, ok)
}

func parseBasicOkOrError(r []string, ok *bool) error {
	//output.Println("parsing:", r)
	if ok != nil {
		*ok = true
	}
	return nil
	/*
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
	*/
}

func parseBasicValuesEndingWithOK(r []string, cmd string, values ...interface{}) error {
	//output.Println("parsing:", r)
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
	return nil
}
