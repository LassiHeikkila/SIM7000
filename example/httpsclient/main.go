package main

import (
	"context"
	"flag"
	"os"

	"github.com/LassiHeikkila/SIM7000/https_native"
	"github.com/LassiHeikkila/SIM7000/output"
)

func init() {
	output.SetWriter(os.Stdout)
}

func main() {
	apnFlag := flag.String("apn", "internet", "Which APN to use when connecting to network")
	deviceFlag := flag.String("device", "/dev/ttyS0", "Which device to talk to module through")
	certFlag := flag.String("cert", "", "Path to certificate")
	flag.Parse()

	if *certFlag == "" {
		output.Println("You must provide a path to a certificate file with -cert flag")
		return
	}

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
		APN: moduleSettings.APN,
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
	headers := map[string]string{
		"accept": "application/json",
	}
	status, data, err := httpsClient.Post(urlToPostTo, []byte(dataToPost), headers, *certFlag)
	output.Printf("Got status %d\n", status)
	if err != nil {
		output.Println("Failed to POST to", urlToPostTo, ":", err)
	} else {
		output.Println("GOT DATA:", string(data))
	}
}
