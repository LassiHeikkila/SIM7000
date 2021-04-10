package http

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"strconv"
	"time"

	"github.com/LassiHeikkila/SIM7000/module"
	"github.com/LassiHeikkila/SIM7000/output"
)

// HttpClient is a struct wrapping the module, implementing HTTP functionality via AT commands
type HttpClient struct {
	module module.Module
}

// Settings is a struct used to configure the HttpClient.
// APN is same APN you would use to configure the Module
// ProxyIP is http proxy IP to use. None used if empty
// ProxyPort is http proxy port to use. None used if 0.
type Settings struct {
	APN       string
	ProxyIP   string
	ProxyPort int
}

// NewClient returns a ready to use HttpClient, given a working Module and working Settings.
// If working HttpClient cannot be created, nil is returned.
func NewClient(module module.Module, settings Settings) *HttpClient {
	c := &HttpClient{module: module}

	output.Println("Setting module to HTTP mode...")

	if gotOK, _ := c.module.SendATCommand("+HTTPINIT", 2*time.Second, "OK"); !gotOK {
		output.Println("HTTP init failed")
		return nil
	}

	time.Sleep(2 * time.Second)

	if settings.APN == "" {
		output.Println("You must provide APN to use for HTTP service")
		return nil
	}

	output.Println("Setting APN for bearer")

	if gotOK, _ := c.module.SendATCommand(fmt.Sprintf("+SAPBR=3,1,\"APN\",\"%s\"", settings.APN), 2*time.Second, "OK"); gotOK {
		output.Println("HTTP bearer APN configured")
	} else {
		output.Println("Failed to configure HTTP bearer APN")
		return nil
	}

	if gotOK, _ := c.module.SendATCommand("+SAPBR=1,1", 2*time.Second, "OK"); gotOK {
		output.Println("Bearer opened successfully")
	} else {
		output.Println("Failed to open bearer")
		return nil
	}

	output.Println("Querying bearer...")
	response, _ := c.module.SendATCommandReturnResponse("+SAPBR=2,1", 2*time.Second)
	output.Println("response:", response)

	time.Sleep(2 * time.Second)

	return c
}

func (c *HttpClient) Close() {
	output.Println("Closing HTTP service")
	gotOK, _ := c.module.SendATCommand("+HTTPTERM", time.Second, "OK")
	if gotOK {
		output.Println("HTTP service terminated with success")
	} else {
		output.Println("Failed to terminate HTTP service")
	}
	gotOK, _ = c.module.SendATCommand("+SAPBR=0,1", time.Second, "OK")
	if gotOK {
		output.Println("HTTP bearer closed with success")
	} else {
		output.Println("Failed to close bearer")
	}
}

func (c *HttpClient) Get(url string) (int, []byte, error) {
	// set CID 1, honestly don't know what this means but SIMCOM documentation says to do it
	output.Println("Setting CID")
	if ok, _ := c.module.SendATCommand("+HTTPPARA=\"CID\",1", 2*time.Second, "OK"); ok {
		output.Println("CID set to 1")
	} else {
		output.Println("Failed to set CID to 1")
		return 0, nil, errors.New("HTTP service configuration failed")
	}

	// set URL
	output.Println("Setting URL")
	if ok, _ := c.module.SendATCommand(fmt.Sprintf("+HTTPPARA=\"URL\",\"%s\"", url), 2*time.Second, "OK"); ok {
		output.Println("URL set to", url)
	} else {
		output.Println("Failed to set URL to", url)
		return 0, nil, errors.New("HTTP service configuration failed")
	}
	// execute GET
	output.Println("Executing GET")
	response, _ := c.module.SendATCommandReturnResponse("+HTTPACTION=0", 10*time.Second)
	output.Println(response)
	actionResponse, err := parseHTTPActionResponse(response)
	if err != nil {
		return 0, nil, err
	}

	var data []byte
	if actionResponse.dataLength > 0 {
		// read
		output.Println("Reading data")
		resp, _ := c.module.SendATCommandReturnResponse("+HTTPREAD", 5*time.Second)
		for _, line := range resp {
			data = append(data, []byte(line + "\n")...)
		}
	}

	return actionResponse.responseCode, data, nil
}

// Post executes a HTTP Post, returning the HTTP status code and any response data or error
func (c *HttpClient) Post(url string, b []byte, headerParams map[string]string) (int, []byte, error) {
	// set CID 1, honestly don't know what this means but SIMCOM documentation says to do it
	output.Println("Setting CID")
	if ok, _ := c.module.SendATCommand("+HTTPPARA=\"CID\",1", 2*time.Second, "OK"); ok {
		output.Println("CID set to 1")
	} else {
		output.Println("Failed to set CID to 1")
		return 0, nil, errors.New("HTTP service configuration failed")
	}

	// set URL
	output.Println("Setting URL")
	if ok, _ := c.module.SendATCommand(fmt.Sprintf("+HTTPPARA=\"URL\",\"%s\"", url), 2*time.Second, "OK"); ok {
		output.Println("URL set to", url)
	} else {
		output.Println("Failed to set URL to", url)
		return 0, nil, errors.New("HTTP service configuration failed")
	}

	if headerParams != nil {
		headerInfo := ""
		for key, value := range headerParams {
			headerInfo += fmt.Sprintf("%s: %s\n", key, value)
		}
		// set header params
		if ok, _ := c.module.SendATCommand(fmt.Sprintf("+HTTPPARA=\"USERDATA\",\"%s\"", headerInfo), 2*time.Second, "OK"); ok {
			output.Println("HEADER set to", headerInfo)
		} else {
			output.Println("Failed to set header")
			return 0, nil, errors.New("Failed to set header")
		}
	}

	output.Println("Sending data to module")
	if downloadReady, _ := c.module.SendATCommand(fmt.Sprintf("+HTTPDATA=%d,%d", len(b), 3000), time.Second, "DOWNLOAD"); downloadReady {
		n, err := c.module.Write(b)
		if err != nil {
			output.Println("Error writing data to module:", err)
			return 0, nil, err
		}
		if n != len(b) {
			output.Printf("Only wrote %d of %d bytes\n", n, len(b))
			return 0, nil, errors.New("Short write")
		}
		resp, _ := c.module.ReadATResponse(time.Second)
		if !bytes.Contains(resp, []byte("OK")) {
			output.Println("Module did not OK written data.")
			return 0, nil, errors.New("Write not OK")
		}
	}

	// execute GET
	output.Println("Executing POST")
	response, _ := c.module.SendATCommandReturnResponse("+HTTPACTION=1", 10*time.Second)
	output.Println(string(response))
	actionResponse, err := parseHTTPActionResponse(response)
	if err != nil {
		output.Println("Error parsing HTTP action response:", err)
		return 0, nil, err
	}

	var data []byte
	if actionResponse.dataLength > 0 {
		// read
		output.Println("Reading data")
		data, _ = c.module.SendATCommandReturnResponse("+HTTPREAD", 5*time.Second)
	}

	return actionResponse.responseCode, data, nil
}

type actionResponse struct {
	action       int
	responseCode int
	dataLength   int
}

func parseHTTPActionResponse(response []string) (actionResponse, error) {
	for _, line := range response {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "+HTTPACTION:") {
			line = strings.TrimPrefix(line, "+HTTPACTION:")
			parts := strings.Split(line, ",")
			if len(parts) == 3 {
				validAct := func(input int64) bool { return input >= 0 && input < 4 }
				validResp := func(input int64) bool { return input > 0 && input < 999 }
				validLen := func(input int64) bool { return input > 0 && input < 319488 } // 319488 is max data size supported by module

				act, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 32)
				if err != nil || !validAct(act) {
					return actionResponse{}, err
				}
				resp, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 32)
				if err != nil || !validResp(resp) {
					return actionResponse{}, err
				}
				dataLen, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 32)
				if err != nil || !validLen(dataLen) {
					return actionResponse{}, err
				}
				return actionResponse{
					action:       int(act),
					responseCode: int(resp),
					dataLength:   int(dataLen),
				}, nil

			}
		}
	}
	return actionResponse{}, errors.New("HTTPACTION response not found")
}
