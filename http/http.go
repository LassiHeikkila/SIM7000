package http

import (
	"bufio"
	nethttp "net/http"
	"strings"

	"github.com/LassiHeikkila/SIM7000/tcp"
	"github.com/LassiHeikkila/SIM7000/module"
)

func NewClient() *nethttp.Client {
	client := nethttp.Client{}
	client.Transport = newRoundTripper()
	if client.Transport == nil {
		return nil
	}
	return &client
}

func NewTransport() *nethttp.Transport {
	transport := nethttp.Transport{}
	transport.Dial = tcp.Dial
	transport.MaxIdleConns = 1
	transport.MaxIdleConnsPerHost = 1
	transport.MaxConnsPerHost = 1
	transport.WriteBufferSize = 1024
	transport.ReadBufferSize = 1024

	return &transport
}

type roundTripper struct {
	module module.Module
}

func newRoundTripper() *roundTripper {
	m, err := tcp.GetModule()
	if err != nil {
		return nil
	}
	return &roundTripper{
		module: m,
	}
}

func (rt roundTripper) RoundTrip(request *nethttp.Request) (*nethttp.Response, error) {
	var host string
	var port string // yes, port is string :) it's just to avoid converting back and forth 
	if strings.Contains(request.URL.Host, ":") {
		parts := strings.Split(request.URL.Host, ":")
		host = parts[0]
		port = parts[1]
	} else {
		host = request.URL.Host
		switch request.URL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	if port == "" {
		port = "80"
	}
	
	url := host + ":" + port
	conn, err := tcp.Dial("tcp4", url)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	err = request.Write(conn)
	if err != nil {
		return nil, err
	}

	resp, err := nethttp.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
