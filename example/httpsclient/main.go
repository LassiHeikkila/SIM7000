package main

import (
	"flag"
	"os"

	"github.com/LassiHeikkila/SIM7000/https"
	"github.com/LassiHeikkila/SIM7000/module"
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

	moduleSettings := module.Settings{
		APN:                   *apnFlag,
		SerialPort:            *deviceFlag,
		MaxConnectionAttempts: 30,
	}

	m := module.NewSIM7000E(moduleSettings)
	if m == nil {
		output.Println("Failed to create working module")
		return
	}
	defer m.Close()

	httpsClientSettings := https.Settings{
		APN: moduleSettings.APN,
	}

	httpsClient := https.NewClient(m, httpsClientSettings)
	if httpsClient == nil {
		output.Println("Failed to create working HTTP client")
		return
	}
	defer httpsClient.Close()

	if err := httpsClient.ConfigureSSL(*certFlag); err != nil {
		output.Println("Error configuring SSL on module:", err.Error())
		return
	}

	headers := map[string]string{
		"accept": "application/json",
	}
	status, data, err := httpsClient.Post(urlToPostTo, []byte(dataToPost), headers)
	output.Printf("Got status %d\n", status)
	if err != nil {
		output.Println("Failed to POST to", urlToPostTo)
	} else {
		output.Println("GOT DATA:", string(data))
	}
}
