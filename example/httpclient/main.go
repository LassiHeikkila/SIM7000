package main

import (
	"flag"
	"os"

	"github.com/LassiHeikkila/SIM7000/http"
	"github.com/LassiHeikkila/SIM7000/module"
	"github.com/LassiHeikkila/SIM7000/output"
)

func init() {
	output.SetWriter(os.Stdout)
}

func main() {
	apnFlag := flag.String("apn", "internet", "Which APN to use when connecting to network")
	deviceFlag := flag.String("device", "/dev/ttyS0", "Which device to talk to module through")
	flag.Parse()

	urlToGet := flag.Arg(0)
	if urlToGet == "" {
		output.Println("Please provide a URL to GET as the first unnamed argument")
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

	httpClientSettings := http.Settings{
		APN: moduleSettings.APN,
	}

	httpClient := http.NewClient(m, httpClientSettings)
	if httpClient == nil {
		output.Println("Failed to create working HTTP client")
		return
	}
	defer httpClient.Close()

	data, err := httpClient.Get(urlToGet)
	if err != nil {
		output.Println("Failed to GET", urlToGet)
	} else {
		output.Println("GOT DATA:", string(data))
	}
}
