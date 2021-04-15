package main

import (
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"log"
	nethttp "net/http"

	"github.com/LassiHeikkila/SIM7000/https_native"
	"github.com/LassiHeikkila/SIM7000/output"
)

func init() {
	output.SetWriter(log.Writer())
}

func main() {
	apnFlag := flag.String("apn", "internet", "Which APN to use when connecting to network")
	deviceFlag := flag.String("device", "/dev/ttyS0", "Which device to talk to module through")
	certFlag := flag.String("cert", "", "Path to certificate")
	flag.Parse()

	//if *certFlag == "" {
	//	output.Println("You must provide a path to a certificate file with -cert flag")
	//	return
	//}

	urlToPostTo := flag.Arg(0)
	if urlToPostTo == "" {
		output.Println("Please provide a URL to POST to as the first unnamed argument")
		return
	}

	dataToPost := flag.Arg(1)
	if dataToPost == "" {
		output.Println("Please provide some data to POST as the second unnamed argument")
		return
	}

	httpsClientSettings := https.Settings{
		APN:         *apnFlag,
		SerialPort:  *deviceFlag,
		CertPath:    *certFlag,
		TraceLogger: log.Default(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpsClient := https.NewClient(ctx, httpsClientSettings)
	if httpsClient == nil {
		output.Println("Failed to create working HTTP client")
		return
	}
	defer httpsClient.Close()
	/*
		if err := httpsClient.UploadCert(*certFlag); err != nil {
			output.Println("Error configuring SSL on module:", err.Error())
			return
		}
	*/
	client := nethttp.Client{
		Transport: httpsClient,
	}
	buf := bytes.NewBuffer([]byte(flag.Arg(1)))
	resp, err := client.Post(flag.Arg(0), "application/json", buf)
	if err != nil {
		output.Println("Failed to POST to", urlToPostTo, ":", err)
	} else {
		output.Println("Response status:", resp.Status)
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			output.Println("Error reading response body:", err)
		} else {
			output.Printf("Response:\n%s\n", string(b))
		}
	}
}
