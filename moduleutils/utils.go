package moduleutils

import (
	"errors"
	"log"
	"time"

	"github.com/warthog618/modem/at"
	"github.com/warthog618/modem/serial"
	"github.com/warthog618/modem/trace"

	"github.com/LassiHeikkila/SIM7000/output"
)

// PowerOff issues AT+CPOWD=1 to the modem
// and returns any error encountered while issuing the command-
func PowerOff(modem *at.AT) error {
	_, err := modem.Command(`+CPOWD=1`)
	return err
}

// Restart issues AT+CPOWD=1 to the modem
// and waits for some time, then pokes the modem to see if it has woken up.
//
// If the modem does not respond after multiple pokes, error is returned.
func Restart(dev string) error {
	{
		p, err := serial.New(serial.WithPort(dev), serial.WithBaud(115200))
		if err != nil {
			return err
		}
		modem := at.New(p, at.WithTimeout(1000*time.Millisecond))
		modem.Command(`+CPOWD=1`)
		p.Close()
		time.Sleep(15 * time.Second)
	}

	p, err := serial.New(serial.WithPort(dev), serial.WithBaud(115200))
	if err != nil {
		return err
	}
	defer p.Close()
	modem := at.New(trace.New(p, trace.WithLogger(log.Default())), at.WithTimeout(500*time.Millisecond))
	for i := 50; i > 0; i-- {
		output.Println("Poke")
		_, err := modem.Command("")
		if err == nil {
			output.Println("Modem responded")
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("Module did not respond after powering it off")
}
