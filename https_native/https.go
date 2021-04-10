package https

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	nethttp "net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/warthog618/modem/at"
	"github.com/warthog618/modem/serial"
	"github.com/warthog618/modem/trace"

	"github.com/LassiHeikkila/SIM7000/output"
)

// HttpsClient is a struct wrapping the module, implementing HTTPS functionality via AT commands
type HttpsClient struct {
	modem    *at.AT
	port     io.ReadWriter
	mutex    sync.Mutex
	certName string
}

// Settings is a struct used to configure the HttpsClient.
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

/*
func defaultHttpsChatScript(settings Settings) module.ChatScript {
	return module.Chatscript{
		Aborts: []string{"ERROR", "BUSY", "NO CARRIER", "+CSQ: 99,99"},
		Commands: []module.CommandResponse{
			module.CommandResponse{"+CFUN=0", "OK", 10 * time.Second, 0},
			module.NormalCommandResponse(fmt.Sprintf(`+CGDCONT=1,"IP","%s"`, settings.APN), "OK"),
			module.CommandResponse{"+CFUN=1", "OK", 10 * time.Second, 0},
			module.NormalCommandResponse(`+CPIN?`, "+CPIN: READY"),
			module.NormalCommandResponse(`+CGATT?`, "+CGATT: 1"),
			module.CommandResponse{`+CNACT=1`, "+APP PDP: ACTIVE", 10 * time.Second, 0},
			module.NormalCommandResponse(`+CNACT?`, "OK"),
		},
	}
}
*/
// NewClient returns a ready to use HttpsClient, given working Settings.
// If working HttpsClient cannot be created, nil is returned.
// HttpsClient implements net/http RoundTripper for HTTP and HTTPS
func NewClient(ctx context.Context, settings Settings) *HttpsClient {
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

	modem := at.New(mio, at.WithTimeout(10*time.Second))

	modem.Init()
	modem.Command("+CFUN=0")
	time.Sleep(10)
	modem.Command(fmt.Sprintf(`+CGDCONT=1,"IP","%s"`, settings.APN))
	time.Sleep(2)
	modem.Command("+CFUN=1")
	time.Sleep(10)
	modem.Command("+CPIN?") // "+CPIN: READY"
	time.Sleep(1)
	modem.Command("+CGATT?") // "+CGATT: 1"
	time.Sleep(1)
	appPdpChan := make(chan struct{})
	appPdpHandler := func(s []string) {
		close(appPdpChan)
	}
	modem.AddIndication("+APP PDP:", appPdpHandler)
	modem.Command("+CNACT=1") // "+APP PDP: ACTIVE" // unsolicited?
	select {
	case <-ctx.Done():
		output.Println("Context cancelled!")
		return nil
	case <-appPdpChan:
	}
	modem.CancelIndication("+APP PDP:")
	modem.Command("+CNACT?") // OK

	c := &HttpsClient{
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

func (c *HttpsClient) Close() {
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

func (c *HttpsClient) RoundTrip(req *nethttp.Request) (*nethttp.Response, error) {
	switch req.URL.Scheme {
	case "http":
		return c.roundTrip(req)
	case "https":
		return c.roundTripHTTPS(req)
	default:
		return nil, errors.New("Unknown scheme")
	}
}

func (c *HttpsClient) roundTrip(req *nethttp.Request) (*nethttp.Response, error) {
	u := fmt.Sprintf("%s:%s", req.URL.Scheme, req.URL.Host)
	if err := c.configure("URL", u); err != nil {
		return nil, err
	}
	if err := c.configure("BODYLEN", 1024); err != nil {
		return nil, err
	}
	if err := c.configure("HEADERLEN", 350); err != nil {
		return nil, err
	}

	if _, err := c.modem.Command("+SHCONN"); err != nil {
		return nil, err
	}

	if _, err := c.modem.Command("+SHSTATE?"); err != nil {
		return nil, err
	}

	if _, err := c.modem.Command("+SHCHEAD"); err != nil {
		return nil, err
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

	err := c.executeRequest(req.Method, *req.URL)
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

func (c *HttpsClient) roundTripHTTPS(req *nethttp.Request) (*nethttp.Response, error) {
	c.modem.Command(`+CSSLCFG="sslversion",1,3`)
	c.modem.Command(fmt.Sprintf(`+SHSSL=1,"%s"`, c.certName)) // empty certName means server cert is not verified

	return c.roundTrip(req)
}

func (c *HttpsClient) configure(key string, value interface{}) error {
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

func (c *HttpsClient) setHeader(key, value string) error {
	_, err := c.modem.Command(fmt.Sprintf(`+SHAHEAD="%s","%s"`, key, value))
	return err
}

func (c *HttpsClient) setParameter(key, value string) error {
	_, err := c.modem.Command(fmt.Sprintf(`+SHPARA="%s","%s"`, key, value))
	return err
}

func (c *HttpsClient) setBody(body string) error {
	_, err := c.modem.Command(fmt.Sprintf(`+SHBOD="%s",%d`, body, len(body)))
	return err
}

func (c *HttpsClient) executeRequest(method string, url url.URL) error {
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

	_, err := c.modem.Command(fmt.Sprintf(`+SHREQ="%s",%d`, url.RequestURI(), methodInt))
	return err
}

func (c *HttpsClient) uploadCert(certPath string) error {
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
	resp, err := c.modem.Command(
		fmt.Sprintf(
			`+CFSWFILE=%d,"%s",0,%d,%d`,
			3,
			certName,
			len(certContents),
			timeoutMs))
	if err != nil {
		return err
	}
	err = parseResponse_CFSWFILE_WRITE(resp) // err == nil when response ends in "DOWNLOAD" so its ready to receive data
	if err != nil {
		return err
	}
	c.port.Write(certContents)

	b := make([]byte, 32)
	if _, err := c.port.Read(b); err != nil || !bytes.Contains(b, []byte("OK")) {
		return errors.New("Failed to write certificate to module filesystem")
	}
	r, err = c.modem.Command("+CFSTERM")
	if err != nil {
		return err
	}
	ok = false
	_ = parseResponse_CFSTERM(r, &ok)
	if !ok {
		return errors.New("Failed to close module filesystem")
	}

	c.certName = certName

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
