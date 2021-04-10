package https

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	nethttp "net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/warthog618/modem/at"
	"github.com/warthog618/modem/serial"
	"github.com/warthog618/modem/trace"

	"github.com/LassiHeikkila/SIM7000/output"
)

// Client is a struct wrapping the module, implementing HTTPS functionality via AT commands
type Client struct {
	modem    *at.AT
	port     io.ReadWriter
	mutex    sync.Mutex
	certName string
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
}

// NewClient returns a ready to use Client, given working Settings.
// If working Client cannot be created, nil is returned.
// Client implements net/http RoundTripper for HTTP and HTTPS
func NewClient(ctx context.Context, settings Settings) *Client {
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

	ready := make(chan struct{})
	readyHandler := func([]string) {
		close(ready)
	}
	modem.AddIndication(`+CPIN: READY`, readyHandler)
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
	modem.AddIndication("+APP PDP:", appPdpHandler)
	output.Println("EXECUTING +CNACT=1")
	if err := checkNoErrorAndResponseOK(modem.Command("+CNACT=1")); err != nil {
		output.Println("CNACT=1 not ok:", err)
		return nil
	}
	select {
	case <-ctx.Done():
		output.Println("Context cancelled!")
		return nil
	case <-appPdpChan:
	}
	modem.CancelIndication("+APP PDP:")
	if !pdpActive {
		output.Println("APP PDP not active")
		return nil
	}
	output.Println("EXECUTING +CNACT?")
	if err := checkNoErrorAndResponseOK(modem.Command("+CNACT?")); err != nil {
		output.Println("CNACT not ok:", err)
		return nil
	}

	c := &Client{
		modem: modem,
		port:  mio,
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
	u := fmt.Sprintf("%s://%s", req.URL.Scheme, req.URL.Host)
	if err := c.configure("URL", u); err != nil {
		return nil, err
	}
	if err := c.configure("BODYLEN", 1024); err != nil {
		return nil, err
	}
	if err := c.configure("HEADERLEN", 350); err != nil {
		return nil, err
	}

	r, err := c.modem.Command("+SHCONN")
	if err != nil {
		return nil, err
	}
	ok := false
	parseResponse_SHCONN(r, &ok)
	if !ok {
		return nil, errors.New("Failed to connect with HTTP")
	}

	r, err = c.modem.Command("+SHSTATE?")
	if err != nil {
		return nil, err
	}
	state := -1
	parseResponse_SHSTATE_READ(r, &state)
	if state != 1 {
		return nil, errors.New("HTTP connection status is not \"connected\"")
	}

	r, err = c.modem.Command("+SHCHEAD")
	if err != nil {
		return nil, err
	}
	ok = false
	parseResponse_SHCHEAD(r, &ok)
	if !ok {
		return nil, errors.New("Failed to clear head")
	}

	for key, values := range req.Header {
		v := strings.Join(values, ",")
		err := c.setHeader(key, v)
		if err != nil {
			return nil, err
		}
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
	}

	var status int
	var dataLen int
	var shreqErr error
	respChan := make(chan struct{})
	handleSHREQ := func(r []string) {
		var t string
		shreqErr = parseResponse_SHREQ_UNSOLICITED_RESPONSE(r, &t, &status, &dataLen)
		close(respChan)
	}

	c.modem.AddIndication("+SHREQ:", handleSHREQ)

	err = c.executeRequest(req.Method, *req.URL)
	if err != nil {
		return nil, err
	}

	select {
	case <-respChan:
	case <-req.Context().Done():
		return nil, errors.New("Context done")
	}

	c.modem.CancelIndication("+SHREQ:")
	if shreqErr != nil {
		return nil, err
	}

	dataRead := 0
	responseData := ""
	allReadChan := make(chan struct{})

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

	c.modem.AddIndication("+SHREAD:", readIndicationHandler)

	if dataLen > 0 {
		_, err := c.modem.Command(fmt.Sprintf(`+SHREAD=0,%d`, dataLen))
		if err != nil {
			return nil, err
		}
	}
	select {
	case <-allReadChan:
	case <-req.Context().Done():
		return nil, errors.New("Context done")
	}

	respReader := strings.NewReader(responseData)
	respReadCloser := ioutil.NopCloser(respReader)

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
	c.modem.Command(`+CSSLCFG="sslversion",1,3`)
	c.modem.Command(fmt.Sprintf(`+SHSSL=1,"%s",`, c.certName)) // empty certName means server cert is not verified

	return c.roundTrip(req)
}

func (c *Client) configure(key string, value interface{}) error {
	switch value.(type) {
	case int:
		_, err := c.modem.Command(fmt.Sprintf(`+SHCONF="%s",%d`, key, value.(int)))
		return err
	case string:
		_, err := c.modem.Command(fmt.Sprintf(`+SHCONF="%s","%s"`, key, value.(string)))
		return err
	default:
		return errors.New("Unhandled value type")
	}
}

func (c *Client) setHeader(key, value string) error {
	var r []string
	var err error
	if r, err = c.modem.Command(fmt.Sprintf(`+SHAHEAD="%s","%s"`, key, value)); err != nil {
		return err
	}
	ok := false
	parseResponse_SHAHEAD_WRITE(r, &ok)
	if !ok {
		return fmt.Errorf(`Failed to set header "%s" to "%s"`, key, value)
	}
	return nil
}

func (c *Client) setParameter(key, value string) error {
	var r []string
	var err error
	if r, err = c.modem.Command(fmt.Sprintf(`+SHPARA="%s","%s"`, key, value)); err != nil {
		return err
	}
	ok := false
	parseResponse_SHPARA_WRITE(r, &ok)
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
	parseResponse_SHBOD_WRITE(r, &ok)
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
