package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/laher/cec"
)

func (a *app) pollMessages() {
	//source := "4f"
	for c := range a.messagesChan {
		if strings.HasPrefix(c, ">> ") {
			log.Printf("plier - cec message rx: %+v", c)
			parts := strings.Split(c[3:], ":")
			if len(parts) < 2 {
				// not a 'receive': ignore
				continue
			}
			switch parts[1] {
			case "8b":
				log.Printf("Vendor button up: %s", parts[2])
				a.report(fmt.Sprintf("vendor button up: %s", parts[2]))
			case "44":
				log.Printf("KEY PRESSED: %s", parts[2])
				switch parts[2] {
				case "44": // play
					if _, err := a.writer.Write([]byte("p")); err != nil {
						panic(err)
					}
				case "46": // pause
					if _, err := a.writer.Write([]byte("p")); err != nil {
						panic(err)
					}
				case "45": // stop
					a.report("stop")
				case "49": // ff
					a.report("fast-forward")
				case "48": // rw
					a.report("rewind")
				case "4b": // f
					a.report("forward")
				case "4c": // b
					a.report("back")
				case "01": // up
					a.report("up")
				case "02": // down
					a.report("down")
				case "03": // left
					a.report("left")
				case "04": // right
					a.report("right")
				default:
					a.report(fmt.Sprintf("key unhandled: %s", parts[2]))
				}
			case "46":
				log.Printf("Requested OSD name - plier")
			//	a.cec.Transmit(fmt.Sprintf("%s:47:70:6c:69:65:72", source))
			case "8c":
				log.Printf("Requested Vendor ID 1582")
				// vendor ID 1582 (Pulse Eight)
			//	a.cec.Transmit(fmt.Sprintf("%s:87:00:15:82", source))
			case "83":
				log.Printf("Requested Physical address")
				// Reply with physical address (playback)
			//	a.cec.Transmit(fmt.Sprintf("%s:84:10:00:04", source))
			case "9f":
				log.Printf("Requested version")
			//	a.cec.Transmit(fmt.Sprintf("%s:9E:00", source))
			case "45":
				log.Printf("Key released: %s", parts[2])
			case "82":
				log.Printf("Active source: %s", parts[2])
			}
		}
	}
}

func (a *app) pollCommands() {
	for c := range a.commandsChan {
		log.Printf("plier - cec command rx: %+v", c)
	}
}

func (a *app) setODSString() {
	if a.isCEC {

		a.cec.SetOSDString(0, "This is Plier")
	}
}

func (a *app) pollKeys() {
	for c := range a.keysChan {
		log.Printf("**************************************************")
		log.Printf("plier - key press rx: %+v", c)
		log.Printf("**************************************************")
	}
}

func (a *app) initCEC() {
	var err error
	a.cec, err = cec.Open("", "cec.go")
	if err != nil {
		log.Panicln("Error starting cec. Try `-cec=false`:", err)
	}
	a.commandsChan = make(chan *cec.Command)
	a.cec.Commands = a.commandsChan
	go a.pollCommands()

	a.keysChan = make(chan int)
	a.cec.KeyPresses = a.keysChan
	go a.pollKeys()

	a.messagesChan = make(chan string)
	a.cec.Messages = a.messagesChan
	go a.pollMessages()

	a.cec.PowerOn(0)

	time.Sleep(5 * time.Second)
	a.cec.SetOSDString(0, "This is Plier")
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()
	for {
		<-ticker.C
		log.Println("Poll device")
		log.Println("---------------------------------")
		a.cec.PollDevice(0)
		//a.cec.Transmit("E0:84:10:00:04")
		log.Println("---------------------------------")
		log.Println("Done: poll device")
	}
}