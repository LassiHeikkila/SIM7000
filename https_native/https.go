package https

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	nethttp "net/http"
	//"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/warthog618/modem/at"
	"github.com/warthog618/modem/serial"
	"github.com/warthog618/modem/trace"

	"github.com/LassiHeikkila/SIM7000/moduleutils"
	"github.com/LassiHeikkila/SIM7000/output"
)

// Client is a struct wrapping the module, implementing HTTPS functionality via AT commands
type Client struct {
	modem    *at.AT
	port     io.ReadWriter
	mutex    sync.Mutex
	certName string

	responseTimeoutDuration time.Duration
	delayBetweenCmds        time.Duration
}

// Settings is a struct used to configure the Client.
// APN is same APN you would use to configure the Module
// ProxyIP is http proxy IP to use. None used if empty
// ProxyPort is http proxy port to use. None used if 0.
type Settings struct {
	APN                   string
	Username              string
	Password              string
	PIN                   string
	SerialPort            string
	MaxConnectionAttempts int
	TraceLogger           *log.Logger

	ProxyIP   string
	ProxyPort int
	CertPath  string

	ResponseTimeoutDuration time.Duration
	DelayBetweenCommands    time.Duration
}

// DefaultResponseTimeoutDuration is how long to wait for a response from server, by default, after sending a request
const DefaultResponseTimeoutDuration = 20 * time.Second

// NewClient returns a ready to use Client, given working Settings.
// If working Client cannot be created, nil is returned.
// Client implements net/http RoundTripper for HTTP and HTTPS
func NewClient(ctx context.Context, settings Settings) *Client {
	output.Println("Restarting modem")
	err := moduleutils.Restart(settings.SerialPort)
	if err != nil {
		output.Println("Failed to restart modem")
		return nil
	}

	output.Println("Initializing module...")

	if settings.APN == "" {
		output.Println("You must provide APN to use for HTTP service")
		return nil
	}

	p, err := serial.New(serial.WithPort(settings.SerialPort), serial.WithBaud(115200))
	if err != nil {
		return nil
	}
	var mio io.ReadWriter
	if settings.TraceLogger != nil {
		mio = trace.New(p, trace.WithLogger(settings.TraceLogger))
	} else {
		mio = p
	}

	modem := at.New(mio, at.WithTimeout(5*time.Second))

	if err := modem.Init(at.WithCmds("E0")); err != nil {
		output.Println("Error initializing modem:", err)
		return nil
	}
	if err := checkNoErrorAndResponseOK(modem.Command("+CFUN=0")); err != nil {
		output.Println("CFUN=0 not ok:", err)
		return nil
	}
	time.Sleep(5 * time.Second)
	if err := checkNoErrorAndResponseOK(modem.Command(fmt.Sprintf(`+CGDCONT=1,"IP","%s"`, settings.APN))); err != nil {
		output.Println("Setting APN not ok:", err)
		return nil
	}

	if err := checkNoErrorAndResponseOK(modem.Command(`+CNMP=38`)); err != nil {
		output.Println("Setting module to LTE only mode failed:", err)
		return nil
	}

	ready := make(chan struct{})
	readyHandler := func([]string) {
		close(ready)
	}
	err = modem.AddIndication(`+CPIN: READY`, readyHandler)
	if err != nil {
		output.Println("Failed to add indication for +CPIN: READY")
		return nil
	}
	defer modem.CancelIndication(`+CPIN: READY`)
	output.Println("EXECUTING +CFUN=1")
	if err := checkNoErrorAndResponseOK(modem.Command("+CFUN=1")); err != nil {
		output.Println("CFUN=1 not ok:", err)
		return nil
	}
	time.Sleep(5 * time.Second)

	select {
	case <-ready:
	case <-ctx.Done():
		output.Println("Context cancelled!")
		return nil
	}
	modem.CancelIndication(`+CPIN: READY`)
	time.Sleep(5 * time.Second)
	output.Println("EXECUTING +CGATT?")
	modem.Command("+CGATT?") // "+CGATT: 1"
	appPdpChan := make(chan struct{})
	pdpActive := false
	appPdpHandler := func(s []string) {
		if s[0] == "+APP PDP: ACTIVE" {
			pdpActive = true
		}
		close(appPdpChan)
	}
	err = modem.AddIndication("+APP PDP:", appPdpHandler)
	if err != nil {
		output.Println("Failed to add indication for +APP PDP:")
		return nil
	}
	defer modem.CancelIndication(`+APP PDP:`)
	output.Println("EXECUTING +CNACT=1")
	if err := checkNoErrorAndResponseOK(modem.Command("+CNACT=1")); err != nil {
		output.Println("CNACT=1 not ok:", err)
		return nil
	}
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	select {
	case <-ctx.Done():
		output.Println("Context cancelled!")
		return nil
	case <-timeout.C:
		output.Println("Command +CNACT failed to respond in time")
		return nil
	case <-appPdpChan: // keep going
	}

	if !pdpActive {
		output.Println("APP PDP not active")
		return nil
	}
	output.Println("EXECUTING +CNACT?")
	if err := checkNoErrorAndResponseOK(modem.Command("+CNACT?")); err != nil {
		output.Println("CNACT not ok:", err)
		return nil
	}
	respTimeout := DefaultResponseTimeoutDuration
	if settings.ResponseTimeoutDuration != 0 {
		respTimeout = settings.ResponseTimeoutDuration
	}
	c := &Client{
		modem:                   modem,
		port:                    mio,
		responseTimeoutDuration: respTimeout,
		delayBetweenCmds:        settings.DelayBetweenCommands,
	}
	if settings.CertPath != "" {
		err := c.uploadCert(settings.CertPath)
		if err != nil {
			output.Println("Failed to upload certificate!")
			return nil
		}
	}

	return c
}

// Close shuts down any open https connections
func (c *Client) Close() {
	output.Println("Closing HTTP service")
	r, err := c.modem.Command("+SHDISC")
	if err != nil {
		output.Println("Error executing +SHDISC")
		return
	}
	ok := false
	_ = parseResponse_SHDISC(r, &ok)
	if !ok {
		output.Println("+SHDISC failed")
		return
	}
	output.Println("HTTP service terminated with success")
}

func (c *Client) wait() {
	if c.delayBetweenCmds != 0 {
		time.Sleep(c.delayBetweenCmds)
	}
}

// RoundTrip executes a http request and returns the response
func (c *Client) RoundTrip(req *nethttp.Request) (*nethttp.Response, error) {
	switch req.URL.Scheme {
	case "http":
		return c.roundTrip(req)
	case "https":
		return c.roundTripHTTPS(req)
	default:
		return nil, errors.New("Unknown scheme")
	}
}

func (c *Client) roundTrip(req *nethttp.Request) (*nethttp.Response, error) {
	//d, _ := httputil.DumpRequest(req, true)
	//output.Println("Request:\n", string(d))
	u := fmt.Sprintf("%s://%s", req.URL.Scheme, req.URL.Host)
	if err := c.configure("URL", u); err != nil {
		return nil, err
	}
	c.wait()
	if err := c.configure("BODYLEN", 1024); err != nil {
		return nil, err
	}
	c.wait()
	if err := c.configure("HEADERLEN", 350); err != nil {
		return nil, err
	}
	c.wait()

	r, err := c.modem.Command("+SHCONN")
	if err != nil {
		return nil, err
	}
	ok := false
	_ = parseResponse_SHCONN(r, &ok)
	if !ok {
		return nil, errors.New("Failed to connect with HTTP")
	}
	defer c.modem.Command("+SHDISC")
	time.Sleep(time.Second)

	r, err = c.modem.Command("+SHSTATE?")
	if err != nil {
		return nil, errors.New("+SHSTATE? returned: " + err.Error())
	}
	state := -1
	_ = parseResponse_SHSTATE_READ(r, &state)
	if state != 1 {
		return nil, errors.New("HTTP connection status is not \"connected\"")
	}
	c.wait()

	r, err = c.modem.Command("+SHCHEAD")
	if err != nil {
		return nil, err
	}
	ok = false
	_ = parseResponse_SHCHEAD(r, &ok)
	if !ok {
		return nil, errors.New("Failed to clear head")
	}
	c.wait()

	for key, values := range req.Header {
		v := strings.Join(values, ",")
		err := c.setHeader(key, v)
		if err != nil {
			return nil, err
		}
		c.wait()
	}

	if req.Body != nil {
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		err = c.setBody(string(b))
		if err != nil {
			return nil, err
		}
		c.wait()
	}

	var status int
	var dataLen int
	var shreqErr error
	respChan := make(chan struct{})
	handleSHREQ := func(r []string) {
		output.Println("handleSHREQ called")
		var t string
		shreqErr = parseResponse_SHREQ_UNSOLICITED_RESPONSE(r, &t, &status, &dataLen)
		log.Println("unsolicited shreq parsed, status", status)
		close(respChan)
	}

	err = c.modem.AddIndication("+SHREQ:", handleSHREQ)
	defer c.modem.CancelIndication("+SHREQ:")
	if err != nil {
		output.Println("error registering handler for SHREQ:", err)
		return nil, err
	}
	c.wait()

	err = c.executeRequest(req.Method, *req.URL)
	if err != nil {
		return nil, err
	}
	c.wait()

	timeout := time.NewTimer(c.responseTimeoutDuration)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		shreqErr = errors.New("no response")
	case <-respChan:
	case <-req.Context().Done():
		return nil, errors.New("context done")
	}

	if shreqErr != nil {
		return nil, shreqErr
	}

	dataRead := 0
	responseData := ""
	allReadChan := make(chan struct{})
	if dataLen > 0 {
		readIndicationHandler := func(r []string) {
			var length int
			var data string
			parseResponse_SHREAD_UNSOLICITED_RESPONSE(r, &data, &length)
			dataRead += length
			responseData += data

			if dataRead >= dataLen {
				close(allReadChan)
			}
		}

		err = c.modem.AddIndication("+SHREAD:", readIndicationHandler)
		if err != nil {
			log.Println("error registering read indication handler:", err)
			return nil, err
		}
		defer c.modem.CancelIndication("+SHREAD:")

		_, err := c.modem.Command(fmt.Sprintf(`+SHREAD=0,%d`, dataLen))
		if err != nil {
			return nil, err
		}
		c.wait()
	} else {
		close(allReadChan)
	}
	select {
	case <-allReadChan:
	case <-req.Context().Done():
		return nil, errors.New("context done")
	}

	var respReadCloser io.ReadCloser
	if len(responseData) > 0 {
		respReader := strings.NewReader(responseData)
		respReadCloser = ioutil.NopCloser(respReader)
	} else {
		respReadCloser = nil
	}

	resp := &nethttp.Response{
		Status:        fmt.Sprintf("%d %s", status, nethttp.StatusText(status)),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          respReadCloser,
		ContentLength: int64(dataLen),
		Request:       req,
	}

	return resp, nil
}

func (c *Client) roundTripHTTPS(req *nethttp.Request) (*nethttp.Response, error) {
	if err := checkNoErrorAndResponseOK(c.modem.Command(`+CSSLCFG="sslversion",1,3`)); err != nil {
		return nil, err
	}
	c.wait()
	// empty certName means server cert is not verified
	if err := checkNoErrorAndResponseOK(c.modem.Command(fmt.Sprintf(`+SHSSL=1,"%s"`, c.certName))); err != nil {
		return nil, err
	}
	c.wait()

	return c.roundTrip(req)
}

func (c *Client) configure(key string, value interface{}) error {
	switch value := value.(type) {
	case int:
		_, err := c.modem.Command(fmt.Sprintf(`+SHCONF="%s",%d`, key, value))
		return err
	case string:
		_, err := c.modem.Command(fmt.Sprintf(`+SHCONF="%s","%s"`, key, value))
		return err
	default:
		return errors.New("Unhandled value type")
	}
}

func (c *Client) setHeader(key, value string) error {
	var r []string
	var err error
	if r, err = c.modem.Command(fmt.Sprintf(`+SHAHEAD="%s","%s"`, key, value)); err != nil {
		return errors.New("+SHAHEAD returned ERROR")
	}
	ok := false
	_ = parseResponse_SHAHEAD_WRITE(r, &ok)
	if !ok {
		return fmt.Errorf(`Failed to set header "%s" to "%s"`, key, value)
	}
	return nil
}

func (c *Client) setParameter(key, value string) error {
	var r []string
	var err error
	if r, err = c.modem.Command(fmt.Sprintf(`+SHPARA="%s","%s"`, key, value)); err != nil {
		return errors.New("+SHPARA returned ERROR")
	}
	ok := false
	_ = parseResponse_SHPARA_WRITE(r, &ok)
	if !ok {
		return fmt.Errorf(`Failed to set parameter "%s" to "%s"`, key, value)
	}
	return nil
}

func (c *Client) setBody(body string) error {
	var r []string
	var err error
	if r, err = c.modem.Command(fmt.Sprintf(`+SHBOD="%s",%d`, strings.ReplaceAll(body, `"`, `\"`), len(body))); err != nil {
		return err
	}
	ok := false
	_ = parseResponse_SHBOD_WRITE(r, &ok)
	if !ok {
		return fmt.Errorf(`Failed to set body to "%s"`, body)
	}
	return nil
}

// executeRequest does not handle the Unsolicited Result Code, it must be handled outside this function
func (c *Client) executeRequest(method string, url url.URL) error {
	methodInt := 0
	switch method {
	case nethttp.MethodGet:
		methodInt = 1
	case nethttp.MethodHead:
		methodInt = 5
	case nethttp.MethodPost:
		methodInt = 3
	case nethttp.MethodPut:
		methodInt = 2
	case nethttp.MethodPatch:
		methodInt = 4
	default:
		return errors.New("Method not supported by SIM7000X: " + method)
	}

	r, err := c.modem.Command(fmt.Sprintf(`+SHREQ="%s",%d`, url.RequestURI(), methodInt))
	if err != nil {
		return err
	}
	ok := false
	_ = parseResponse_SHREQ_WRITE(r, &ok)
	if !ok {
		return errors.New("Request execution failed")
	}
	return nil
}

func (c *Client) uploadCert(certPath string) error {
	output.Println("Storing certificate on module filesystem")
	r, err := c.modem.Command("+CFSINIT")
	if err != nil {
		return err
	}
	ok := false
	_ = parseResponse_CFSINIT(r, &ok)
	if !ok {
		return errors.New("Module filesystem initialization failed")
	}
	defer c.modem.Command("+CFSTERM")

	const maxFileSize = 10240
	const timeoutMs = 1000
	certContents, err := ioutil.ReadFile(certPath)
	certName := "root.pem"
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
	downloadDone := make(chan struct{})
	downloadHandler := func([]string) {
		c.port.Write(certContents)
		close(downloadDone)
	}
	c.modem.AddIndication("DOWNLOAD", downloadHandler)
	defer c.modem.CancelIndication("DOWNLOAD")

	ctx, cancel := context.WithTimeout(context.Background(), timeoutMs*time.Millisecond)
	defer cancel()

	c.modem.Command(
		fmt.Sprintf(
			`+CFSWFILE=%d,"%s",0,%d,%d`,
			3,
			certName,
			len(certContents),
			timeoutMs))

	select {
	case <-downloadDone:
	case <-ctx.Done():
		return errors.New("Failed to upload cert")
	}

	c.certName = certName

	if err := checkNoErrorAndResponseOK(c.modem.Command(fmt.Sprintf(`+CSSLCFG="convert",2,"%s"`, c.certName))); err != nil {
		return err
	}

	return nil
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
