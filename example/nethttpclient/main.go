package main

import (
	"fmt"
	nethttp "net/http"
	"io/ioutil"
	"os"

	"github.com/LassiHeikkila/SIM7000/tcp"
	"github.com/LassiHeikkila/SIM7000/http"
	"github.com/LassiHeikkila/SIM7000/output"
)

func main() {
	output.SetWriter(os.Stdout)
	fmt.Println("Registering settings")
	tcp.RegisterSetting("APN", "internet")
	tcp.RegisterSetting("PORT", "/dev/ttyS0")
	tcp.RegisterSetting("DEBUG", "")

	fmt.Println("Creating new transport")
	transport := http.NewTransport()
	if transport == nil {
		panic("Nil transport")
	}

	fmt.Println("Creating HTTP client")
	client := &nethttp.Client{
		Transport: transport,
	}

	url := "http://example.com"
	fmt.Println("GET", url)
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Got response: %+v\n", resp)
	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Response body:\n", string(b))
}