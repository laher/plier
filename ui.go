package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/laher/cec"
)

var (
	mediaPlayer = flag.String("media-player", "omxplayer", "media player executable for movies etc")
	useCec      = flag.Bool("cec", true, "use cec for HDMI support")
	walkDir     = flag.Bool("walk", true, "walk dir (rather than ls dir)")
	onlyMedia   = flag.Bool("only-media", true, fmt.Sprintf("only list media files (%v)", omxFiletypes))
)

type app struct {
	pwd          string
	writer       io.Writer
	walkDir      bool
	commandsChan chan *cec.Command
	keysChan     chan int
	messagesChan chan string
	g            *gocui.Gui

	lines []string
	cec   *cec.Connection
}

func main() {
	flag.Parse()
	w, err := os.Create("/tmp/plier.log")
	if err != nil {
		log.Panicln(err)
	}
	log.SetOutput(w)
	a := &app{walkDir: *walkDir}
	if *useCec {
		go func() {
			a.cec, err = cec.Open("", "cec.go")
			if err != nil {
				log.Panicln("Error starting cec:", err)
			}
			ch := make(chan *cec.Command)
			a.commandsChan = ch
			a.cec.Commands = ch
			go a.pollCommands()

			chKeys := make(chan int)
			a.keysChan = chKeys
			a.cec.KeyPresses = chKeys
			go a.pollKeys()

			chMessages := make(chan string)
			a.messagesChan = chMessages
			a.cec.Messages = chMessages
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
		}()
	}
	a.g, err = gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer a.g.Close()
	if len(flag.Args()) > 0 {
		a.pwd = flag.Args()[0]
	} else {
		a.pwd, err = os.Getwd()
		if err != nil {
			log.Panicln(err)
		}
	}
	a.g.SetManagerFunc(a.layout)
	if err := setKeybindings(a); err != nil {
		log.Panicln(err)
	}
	if err := a.g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
	a.cec.Destroy()
}

func nextView(g *gocui.Gui, v *gocui.View) error {
	v.SelBgColor = gocui.ColorWhite
	if v == nil || v.Name() == "side" {
		if _, err := g.SetCurrentView("main"); err != nil {
			return err
		}

		mv, err := g.View("main")
		if err == nil {
			mv.SelBgColor = gocui.ColorBlue
		}
		return err
	}
	if _, err := g.SetCurrentView("side"); err != nil {
		return err
	}
	sv, err := g.View("side")
	if err == nil {
		sv.SelBgColor = gocui.ColorBlue
	}
	return err
}

func setKeybindings(a *app) error {
	g := a.g
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, quit); err != nil {
		return err
	}
	for _, v := range []string{"main", "side", "top"} {
		if err := g.SetKeybinding(v, gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
			return err
		}
		if err := g.SetKeybinding(v, gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
			return err
		}
		if err := g.SetKeybinding(v, gocui.KeyTab, gocui.ModNone, nextView); err != nil {
			return err
		}
		if err := g.SetKeybinding(v, gocui.KeyCtrlW, gocui.ModNone, nextView); err != nil {
			return err
		}
		if err := g.SetKeybinding(v, gocui.KeyEnter, gocui.ModNone, a.selectItem); err != nil {
			return err
		}

		if err := g.SetKeybinding(v, gocui.KeyCtrlR, gocui.ModNone, a.reload); err != nil {
			return err
		}
	}
	return nil
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+1); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
	}
	return nil
}

var omxFiletypes = []string{".mkv", ".mp4", ".m4v", ".avi", "mp3"}

func (a *app) queueReload() {
	a.g.Update(func(g *gocui.Gui) error {
		return a.reload(g, nil)
	})
}

func (a *app) reload(g *gocui.Gui, _ *gocui.View) error {
	t, _ := g.View("top")
	err := a.refreshTop(t)
	if err != nil {
		return err
	}
	s, _ := g.View("side")
	err = a.refreshSide(s)
	if err != nil {
		return err
	}
	m, _ := g.View("main")
	err = a.refreshMain(m)
	return err

}

func (a *app) selectItem(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		switch v.Name() {
		case "main":
			_, y := v.Cursor()
			item, err := v.Line(y)
			if err != nil {
				return err
			}
			exe := "xdg-open"
			for _, ft := range omxFiletypes {
				if strings.HasSuffix(item, ft) {
					exe = *mediaPlayer
				}
			}
			if !a.walkDir {
				item = filepath.Join(a.pwd, item)
			}
			if a.cec != nil {
				a.cec.SetOSDString(0, "This is Plier")
			}
			cmd := exec.Command(exe, item)
			a.writer, err = cmd.StdinPipe()
			if err != nil {
				return err
			}

			err = cmd.Start()
			if err != nil {
				return err
			}
		case "side":
			_, y := v.Cursor()
			item, err := v.Line(y)
			if err != nil {
				return err
			}
			a.pwd = filepath.Join(a.pwd, item)

			mainV, err := g.View("main")
			if err != nil {
				return err

			}
			err = a.refreshMain(mainV)
			if err != nil {
				return err
			}
			return a.refreshSide(v)
		}
	}
	return nil
}

func (a *app) refreshTop(v *gocui.View) error {
	if v == nil {
		var err error
		v, err = a.g.View("top")
		if err != nil {
			return err
		}
	}
	v.Clear()
	fmt.Fprintf(v, "--- Plier --- [%s] [%d]\n", time.Now().Format("03:04:05"), len(a.lines))
	if len(a.lines) > 3 {
		a.lines = a.lines[len(a.lines)-3:]
	}
	for _, line := range a.lines {
		fmt.Fprintln(v, line)
	}
	v.SetCursor(0, 0)
	return nil
}
func (a *app) refreshSide(v *gocui.View) error {
	files, err := ioutil.ReadDir(a.pwd)
	if err != nil {
		return err
	}
	v.Clear()
	fmt.Fprintln(v, "..")
	for _, f := range files {
		if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			fmt.Fprintln(v, f.Name())
		}
	}
	v.SetCursor(0, 0)
	return nil
}

func (a *app) pollCommands() {
	for c := range a.commandsChan {
		log.Printf("plier - cec command rx: %+v", c)
	}
}

func (a *app) pollKeys() {
	for c := range a.keysChan {
		log.Printf("**************************************************", c)
		log.Printf("plier - key press rx: %+v", c)
		log.Printf("**************************************************", c)
	}
}

func (a *app) report(line string) {
	a.lines = append(a.lines, line)
	a.queueReload()
}

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

const maxDirs = 50

func (a *app) walkMain(v *gocui.View) error {
	dirCount := 0
	err := filepath.Walk(a.pwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			//fmt.Printf("skipping a dir without errors: %+v \n", info.Name())
			return filepath.SkipDir
		}
		if info.IsDir() {
			dirCount++
			if dirCount > maxDirs {
				return filepath.SkipDir
			}
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			if *onlyMedia {
				for _, ft := range omxFiletypes {
					if strings.HasSuffix(info.Name(), ft) {
						fmt.Fprintln(v, path)
						break
					}
				}
			} else {
				fmt.Fprintln(v, path)
			}
		}
		return nil
	})
	return err
}

func (a *app) refreshMain(v *gocui.View) error {
	v.Clear()
	if a.walkDir {
		return a.walkMain(v)
	}
	return a.lsMain(v)
}

func (a *app) lsMain(v *gocui.View) error {
	files, err := ioutil.ReadDir(a.pwd)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			fmt.Fprintln(v, f.Name())
		}
	}
	v.SetCursor(0, 0)
	return nil
}

func (a *app) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	sideWidth := maxX / 3

	if v, err := g.SetView("top", 0, 0, maxX, 5); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorBlue
		v.SelFgColor = gocui.ColorBlack
		a.refreshTop(v)
	}
	if v, err := g.SetView("side", 0, 5, sideWidth, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorBlue
		v.SelFgColor = gocui.ColorBlack
		a.refreshSide(v)
		if _, err := g.SetCurrentView("side"); err != nil {
			log.Panicln(err)
		}
	}
	if v, err := g.SetView("main", sideWidth, 5, maxX-10, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.SelFgColor = gocui.ColorBlack
		a.refreshMain(v)
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
