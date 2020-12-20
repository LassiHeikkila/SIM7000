package http

import (
	"errors"
	"fmt"
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

// New returns a ready to use HttpClient, given a working Module and working Settings.
// If working HttpClient cannot be created, nil is returned.
func NewClient(module module.Module, settings Settings) *HttpClient {
	c := &HttpClient{module: module}

	output.Println("Setting module to HTTP mode...")

	if gotOK, _ := c.module.SendATCommand("AT+HTTPINIT", 2*time.Second, "OK"); !gotOK {
		output.Println("HTTP init failed")
		return nil
	}

	time.Sleep(2 * time.Second)

	if settings.APN == "" {
		output.Println("You must provide APN to use for HTTP service")
		return nil
	}

	output.Println("Setting APN for bearer")

	if gotOK, _ := c.module.SendATCommand(fmt.Sprintf("AT+SAPBR=3,1,\"APN\",\"%s\"", settings.APN), 2*time.Second, "OK"); gotOK {
		output.Println("HTTP bearer APN configured")
	} else {
		output.Println("Failed to configure HTTP bearer APN")
		return nil
	}

	if gotOK, _ := c.module.SendATCommand("AT+SAPBR=1,1", 2*time.Second, "OK"); gotOK {
		output.Println("Bearer opened successfully")
	} else {
		output.Println("Failed to open bearer")
		return nil
	}

	output.Println("Querying bearer...")
	response, _ := c.module.SendATCommandReturnResponse("AT+SAPBR=2,1", 2*time.Second)
	output.Println("response:", string(response))

	time.Sleep(2 * time.Second)

	return c
}

func (c *HttpClient) Close() {
	output.Println("Closing HTTP service")
	gotOK, _ := c.module.SendATCommand("AT+HTTPTERM", time.Second, "OK")
	if gotOK {
		output.Println("HTTP service terminated with success")
	} else {
		output.Println("Failed to terminate HTTP service")
	}
	gotOK, _ = c.module.SendATCommand("AT+SAPBR=0,1", time.Second, "OK")
	if gotOK {
		output.Println("HTTP bearer closed with success")
	} else {
		output.Println("Failed to close bearer")
	}
}

func (c *HttpClient) Get(url string) ([]byte, error) {
	// set CID 1, honestly don't know what this means but SIMCOM documentation says to do it
	output.Println("Setting CID")
	if ok, _ := c.module.SendATCommand("AT+HTTPPARA=\"CID\",1", 2*time.Second, "OK"); ok {
		output.Println("CID set to 1")
	} else {
		output.Println("Failed to set CID to 1")
		return nil, errors.New("HTTP service configuration failed")
	}

	// set URL
	output.Println("Setting URL")
	if ok, _ := c.module.SendATCommand(fmt.Sprintf("AT+HTTPPARA=\"URL\",\"%s\"", url), 2*time.Second, "OK"); ok {
		output.Println("URL set to", url)
	} else {
		output.Println("Failed to set URL to", url)
		return nil, errors.New("HTTP service configuration failed")
	}
	time.Sleep(2)
	// execute GET
	output.Println("Executing GET")
	response, _ := c.module.SendATCommandReturnResponse("AT+HTTPACTION=0", 10*time.Second)
	output.Println(string(response))

	// read
	output.Println("Reading data")
	data, _ := c.module.SendATCommandReturnResponse("AT+HTTPREAD", 5*time.Second)

	return data, nil
}
