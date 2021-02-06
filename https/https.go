package https

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/LassiHeikkila/SIM7000/module"
	"github.com/LassiHeikkila/SIM7000/output"
)

// HttpsClient is a struct wrapping the module, implementing HTTPS functionality via AT commands
type HttpsClient struct {
	module module.Module
}

// Settings is a struct used to configure the HttpsClient.
// APN is same APN you would use to configure the Module
// ProxyIP is http proxy IP to use. None used if empty
// ProxyPort is http proxy port to use. None used if 0.
type Settings struct {
	APN       string
	ProxyIP   string
	ProxyPort int
	CertPath  string
}

// NewClient returns a ready to use HttpsClient, given a working Module and working Settings.
// If working HttpsClient cannot be created, nil is returned.
func NewClient(m module.Module, settings Settings) *HttpsClient {
	c := &HttpsClient{module: m}

	output.Println("Setting module to HTTP mode...")

	time.Sleep(2 * time.Second)

	if settings.APN == "" {
		output.Println("You must provide APN to use for HTTP service")
		return nil
	}

	script := module.ChatScript{
		Aborts: []string{"ERROR", "BUSY", "NO CARRIER", "+CSQ: 99,99"},
		Commands: []module.CommandResponse{
			module.CommandResponse{"AT", "OK", time.Second, 10},
			module.CommandResponse{"AT+CFUN=0", "OK", 10 * time.Second, 0},
			module.CommandResponse{"AT", "OK", time.Second, 10},
			module.NormalCommandResponse(fmt.Sprintf(`AT+CGDCONT=1,"IP","%s"`, settings.APN), "OK"),
			module.CommandResponse{"AT+CFUN=1", "OK", 10 * time.Second, 0},
			module.NormalCommandResponse(`AT+CPIN?`, "+CPIN: READY"),
			module.NormalCommandResponse(`AT+CGATT?`, "+CGATT: 1"),
			module.CommandResponse{`AT+CNACT=1`, "+APP PDP: ACTIVE", 10 * time.Second, 0},
			module.NormalCommandResponse(`AT+CNACT?`, "OK"),
		},
	}

	_, err := c.module.RunChatScript(script)
	if err != nil {
		output.Println("Error initializing module:", err)
		return nil
	}
	return c
}

func (c *HttpsClient) Close() {
	output.Println("Closing HTTP service")
	gotOK, _ := c.module.SendATCommand("AT+SHDISC", time.Second, "OK")
	if gotOK {
		output.Println("HTTP service terminated with success")
	} else {
		output.Println("Failed to terminate HTTP service")
	}
}

func (c *HttpsClient) UploadCert(certPath string) error {
	output.Println("Storing certificate on module filesystem")
	if gotOK, _ := c.module.SendATCommand("AT+CFSINIT", time.Second, "OK"); !gotOK {
		return errors.New("Unable to use module filesystem")
	}
	const maxFileSize = 10240
	const timeoutMs = 3000
	certContents, err := ioutil.ReadFile(certPath)
	certName := path.Base(certPath)
	if err != nil {
		return errors.New("Unable to read certificate file: " + err.Error())
	}
	if len(certContents) > maxFileSize {
		return fmt.Errorf(
			"Certificate is too big (%d bytes) for module filesystem, max allowed is %d",
			len(certContents),
			maxFileSize,
		)
	}
	if downloadReady, _ := c.module.SendATCommand(
		fmt.Sprintf(
			`AT+CFSWFILE=%d,"%s",0,%d,%d`,
			3,
			certName,
			len(certContents),
			timeoutMs),
		time.Second,
		"DOWNLOAD"); !downloadReady {
		return errors.New("Unable to write certificate to module filesystem")
	}
	c.module.Write(certContents)
	if resp, err := c.module.ReadATResponse(timeoutMs * time.Millisecond); err != nil || !bytes.Contains(resp, []byte("OK")) {
		return errors.New("Failed to write certificate to module filesystem")
	}
	c.module.SendATCommand("AT+CFSTERM", time.Second, "OK")

	return nil
}

func (c *HttpsClient) configureSSL(atcmd string) error {
	if gotOK, _ := c.module.SendATCommand(
		atcmd,
		time.Second,
		"OK",
	); !gotOK {
		return errors.New("Failed to configure")
	}
	return nil
}

func (c *HttpsClient) Get(url string, certName string) (int, []byte, error) {
	// documentation says the options are
	//		1 QAPI_NET_SSL_CERTIFICATE_E
	//		2 QAPI_NET_SSL_CA_LIST_E
	//		3 QAPI_NET_SSL_PSK_TABLE_E
	// and the example uses 2, so let's go with that for now
	const sslType = 2
	if err := c.configureSSL(fmt.Sprintf(`AT+CSSLCFG="convert",%d,"%s"`, sslType, certName)); err != nil {
		return 0, nil, errors.New("Failed to convert certificate")
	}
	if err := c.configureSSL(fmt.Sprintf(`AT+CSSLCFG="sslversion",%d,%d"`, 1, 3)); err != nil {
		return 0, nil, errors.New("Failed to set sslversion")
	}

	if gotOK, _ := c.module.SendATCommand(
		//fmt.Sprintf(`AT+SHSSL=1,"%s"`, certName),
		`AT+SHSSL=1,""`,
		time.Second,
		"OK",
	); !gotOK {
		return 0, nil, errors.New("Failed to set configure certificate")
	}

	// set URL
	output.Println("Setting URL")
	// strip path from url, i.e. https://somesite.org/some/path --> https://somesite.org
	idx := strings.Index(url, "://")
	start := 0
	end := len(url)
	firstNonSchemeSlash := strings.Index(url[idx+3:], "/")
	if firstNonSchemeSlash != -1 {
		end = start + idx + 3 + firstNonSchemeSlash
	}
	output.Println("Setting URL")
	if ok, _ := c.module.SendATCommand(fmt.Sprintf("AT+SHCONF=\"URL\",\"%s\"", url[start:end]), 2*time.Second, "OK"); ok {
		output.Println("URL set to", url)
	} else {
		output.Println("Failed to set URL to", url)
		return 0, nil, errors.New("HTTP service configuration failed")
	}
	// set BODYLEN
	output.Println("Setting BODYLEN")
	if ok, _ := c.module.SendATCommand(fmt.Sprintf("AT+SHCONF=\"BODYLEN\",\"%d\"", 1024), 2*time.Second, "OK"); !ok {
		output.Println("Failed to set BODYLEN")
		return 0, nil, errors.New("HTTP service configuration failed")
	}
	// set HEADERLEN
	output.Println("Setting HEADERLEN")
	if ok, _ := c.module.SendATCommand(fmt.Sprintf("AT+SHCONF=\"HEADERLEN\",\"%d\"", 350), 2*time.Second, "OK"); !ok {
		output.Println("Failed to set HEADERLEN")
		return 0, nil, errors.New("HTTP service configuration failed")
	}
	// execute GET
	output.Println("Executing GET")
	if ok, _ := c.module.SendATCommand("AT+SHCONN", time.Second, "OK"); !ok {
		output.Println("Failed to connect")
		return 0, nil, errors.New("Connect failed")
	}

	if connectState, _ := c.module.SendATCommand("A+SHSTATE?", time.Second, "+SHSTATE: 1"); !connectState {
		output.Println("Wrong connect state")
		return 0, nil, errors.New("Connection state wrong")
	}

	if ok, _ := c.module.SendATCommand("AT+SHCHEAD", time.Second, "OK"); !ok {
		output.Println("Failed to clear header")
		return 0, nil, errors.New("Failed to clear header")
	}

	if ok, _ := c.module.SendATCommand("AT+SHCPARA", time.Second, "OK"); !ok {
		output.Println("Failed to clear body content")
	}

	response, _ := c.module.SendATCommandReturnResponse(fmt.Sprintf(`AT+SHREQ="%s",1`, url[end:]), time.Second)
	output.Println(string(response))
	shreqResponse, err := parseSHREQResponse(response)
	if err != nil {
		return 0, nil, err
	}

	var data []byte
	if shreqResponse.dataLength > 0 {
		// read
		output.Println("Reading data")
		data, _ = c.module.SendATCommandReturnResponse(fmt.Sprintf("AT+SHREAD=0,%d", shreqResponse.dataLength), 5*time.Second)
	}

	_, _ = c.module.SendATCommand(`AT+SHDISC`, time.Second, "OK")

	return shreqResponse.responseCode, data, nil
}

// Post executes a HTTP Post, returning the HTTP status code and any response data or error
func (c *HttpsClient) Post(url string, b []byte, headerParams map[string]string, certName string) (int, []byte, error) {
	// documentation says the options are
	//		1 QAPI_NET_SSL_CERTIFICATE_E
	//		2 QAPI_NET_SSL_CA_LIST_E
	//		3 QAPI_NET_SSL_PSK_TABLE_E
	// and the example uses 2, so let's go with that for now
	//const sslType = 2
	//if err := c.configureSSL(fmt.Sprintf(`AT+CSSLCFG="convert",%d,"%s"`, sslType, certName)); err != nil {
	//	return 0, nil, errors.New("Failed to convert certificate")
	//}
	if err := c.configureSSL(fmt.Sprintf(`AT+CSSLCFG="sslversion",%d,%d`, 1, 3)); err != nil {
		return 0, nil, errors.New("Failed to set sslversion")
	}

	if gotOK, _ := c.module.SendATCommand(
		//fmt.Sprintf(`AT+SHSSL=1,"%s"`, certName),
		`AT+SHSSL=1,""`,
		time.Second,
		"OK",
	); !gotOK {
		return 0, nil, errors.New("Failed to set configure certificate")
	}

	// set URL
	output.Println("Setting URL")
	// strip path from url, i.e. https://somesite.org/some/path --> https://somesite.org
	idx := strings.Index(url, "://")
	start := 0
	end := len(url)
	firstNonSchemeSlash := strings.Index(url[idx+3:], "/")
	if firstNonSchemeSlash != -1 {
		end = start + idx + 3 + firstNonSchemeSlash
	}
	if ok, _ := c.module.SendATCommand(fmt.Sprintf(`AT+SHCONF="URL","%s"`, url[start:end]), 2*time.Second, "OK"); ok {
		output.Println("URL set to", url[start:end])
	} else {
		output.Println("Failed to set URL to", url[start:end])
		return 0, nil, errors.New("HTTP service configuration failed")
	}
	// set BODYLEN
	output.Println("Setting BODYLEN")
	if ok, _ := c.module.SendATCommand(fmt.Sprintf(`AT+SHCONF="BODYLEN",%d`, 1024), 2*time.Second, "OK"); !ok {
		output.Println("Failed to set BODYLEN")
		return 0, nil, errors.New("HTTP service configuration failed")
	}
	// set HEADERLEN
	output.Println("Setting HEADERLEN")
	if ok, _ := c.module.SendATCommand(fmt.Sprintf(`AT+SHCONF="HEADERLEN",%d`, 350), 2*time.Second, "OK"); !ok {
		output.Println("Failed to set HEADERLEN")
		return 0, nil, errors.New("HTTP service configuration failed")
	}

	if ok, _ := c.module.SendATCommand("AT+SHCONN", time.Second, "OK"); !ok {
		output.Println("Failed to connect")
		return 0, nil, errors.New("Connect failed")
	}

	if connectState, _ := c.module.SendATCommand("A+SHSTATE?", time.Second, "+SHSTATE: 1"); !connectState {
		output.Println("Wrong connect state")
		return 0, nil, errors.New("Connection state wrong")
	}

	if ok, _ := c.module.SendATCommand("AT+SHCHEAD", time.Second, "OK"); !ok {
		output.Println("Failed to clear header")
	}

	if ok, _ := c.module.SendATCommand("AT+SHCPARA", time.Second, "OK"); !ok {
		output.Println("Failed to clear body content")
	}

	if headerParams != nil {
		if _, contentLenSet := headerParams["Content-Length"]; !contentLenSet {
			headerParams["Content-Length"] = fmt.Sprintf("%d", len(b))
		}
		for key, value := range headerParams {
			if gotOK, _ := c.module.SendATCommand(
				fmt.Sprintf(`AT+SHAHEAD="%s","%s"`, key, value),
				time.Second,
				"OK",
			); !gotOK {
				output.Println("Failed to set header key:", key)
			}
		}
	}

	output.Println("Writing body")
	if gotOK, _ := c.module.SendATCommand(fmt.Sprintf(`AT+SHBOD="%s",%d`, string(b), len(b)), time.Second, "OK"); gotOK {
		output.Println("Body written OK")
	} else {
		output.Println("Failed to write body!")
		return 0, nil, errors.New("Failed to write request body")
	}

	// execute POST
	output.Println("Executing POST")
	response, _ := c.module.SendATCommandReturnResponse(fmt.Sprintf(`AT+SHREQ="%s",3`, url[end:]), time.Second)
	output.Println(string(response))
	shreqResponse, err := parseSHREQResponse(response)
	if err != nil {
		return 0, nil, err
	}

	var data []byte
	if shreqResponse.dataLength > 0 {
		// read
		output.Println("Reading data")
		data, _ = c.module.SendATCommandReturnResponse(fmt.Sprintf("AT+SHREAD=0,%d", shreqResponse.dataLength), 5*time.Second)
	}

	_, _ = c.module.SendATCommand(`AT+SHDISC`, time.Second, "OK")

	return shreqResponse.responseCode, data, nil
}

type method int8

const (
	invalid method = 0
	get     method = 1
	put     method = 2
	post    method = 3
	patch   method = 4
	head    method = 5
)

func stringToMethod(str string) method {
	switch strings.ToUpper(strings.Trim(strings.TrimSpace(str), `"`)) {
	case "GET":
		return get
	case "PUT":
		return put
	case "POST":
		return post
	case "PATCH":
		return patch
	case "HEAD":
		return head
	default:
		return invalid
	}
}

type requestResponse struct {
	action       method
	responseCode int
	dataLength   int
}

func parseSHREQResponse(b []byte) (requestResponse, error) {
	lines := bytes.Split(b, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("+SHREQ:")) {
			line = bytes.TrimPrefix(line, []byte("+SHREQ:"))
			parts := bytes.Split(line, []byte(","))
			if len(parts) == 3 {
				act := string(bytes.TrimSpace(parts[0]))
				resp, err := strconv.ParseInt(string(bytes.TrimSpace(parts[1])), 10, 64)
				if err != nil {
					return requestResponse{}, err
				}
				dataLen, err := strconv.ParseInt(string(bytes.TrimSpace(parts[2])), 10, 64)
				if err != nil {
					return requestResponse{}, err
				}
				return requestResponse{
					action:       stringToMethod(act),
					responseCode: int(resp),
					dataLength:   int(dataLen),
				}, nil

			}
		}
	}
	return requestResponse{}, errors.New("HTTPACTION response not found")
}
