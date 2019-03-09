package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
)

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
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, a.quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, a.quit); err != nil {
		return err
	}

	if err := g.SetKeybinding("", 'r', gocui.ModNone, a.reload); err != nil {
		return err
	}

	if err := g.SetKeybinding("", 'p', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		a.report("pause via keypress")
		return a.player.pause()
	}); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 's', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		a.report("stop via keypress")
		return a.player.stop()
	}); err != nil {
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

func (a *app) queueReload() {
	a.g.Update(func(g *gocui.Gui) error {
		return a.reload(g, nil)
	})
}

type button struct {
	id      string
	text    string
	handler func(g *gocui.Gui, v *gocui.View) error
}

func (a *app) buttons() []button {
	var buttons = []button{{
		id:   "play",
		text: "Play",
		handler: func(g *gocui.Gui, v *gocui.View) error {
			return a.player.play()
		},
	}, {
		id:   "pause",
		text: "Pause",
		handler: func(g *gocui.Gui, v *gocui.View) error {
			return a.player.pause()
		},
	}, {
		id:   "stop",
		text: "Stop",
		handler: func(g *gocui.Gui, v *gocui.View) error {
			return a.player.stop()
		},
	}}
	return buttons
}
func (a *app) reload(g *gocui.Gui, _ *gocui.View) error {
	t, _ := g.View("top")
	err := a.refreshTop(t)
	if err != nil {
		return err
	}
	/*
		s, _ := g.View("side")
		err = a.refreshSide(s)
		if err != nil {
			return err
		}
		m, _ := g.View("main")
		err = a.refreshMain(m)
		if err != nil {
			return err
		}
	*/
	return nil
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

			if !a.walkDir {
				item = filepath.Join(a.pwd, item)
			}
			if a.player.supports(item) {
				if a.player.cmd != nil {
					err := a.player.stop()
					if err != nil {
						a.report(fmt.Sprintf("error stopping old item: %s", err))
					}
				}
				ctx := context.Background()
				go a.poll(ctx)
				err := a.player.start(item)
				if err != nil {
					return err
				}
			} else {
				// todo: different command? can't remember if xdg-open is universal on raspbian
				exe := "xdg-open"
				err := exec.Command(exe, item).Run()
				if err != nil {
					return err
				}
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
	fmt.Fprintf(v, "--- Plier [%s] ---\n", time.Now().Format("03:04:05"))
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

func (a *app) report(line string) {
	a.lines = append(a.lines, line)
	a.queueReload()
}

func (a *app) refreshMain(v *gocui.View) error {
	v.Clear()
	if a.walkDir {
		return a.walkMain(v)
	}
	err := a.lsMain(v)
	if err != nil {
		return err
	}
	v.SetCursor(0, 0)
	return nil
}
func (a *app) initCUI() {
	var err error
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
}

func (a *app) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	sideWidth := maxX / 3

	if v, err := g.SetView("top", 0, 0, maxX-10, 5); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorWhite
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
	for i, btn := range a.buttons() {
		if v, err := g.SetView(btn.id, maxX-10, i*2, maxX, (2*i)+2); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Highlight = true
			v.SelBgColor = gocui.ColorCyan
			v.SelFgColor = gocui.ColorBlack
			fmt.Fprintln(v, btn.text)

			if err := g.SetKeybinding(btn.id, gocui.MouseLeft, gocui.ModNone, btn.handler); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *app) quit(g *gocui.Gui, v *gocui.View) error {
	go a.player.stop()
	return gocui.ErrQuit
}
