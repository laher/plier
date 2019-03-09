package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	mediaPlayer = flag.String("media-player", "omxplayer", "media player executable for movies etc")
	useCec      = flag.Bool("cec", true, "use cec for HDMI support")
	walkDir     = flag.Bool("walk", true, "walk dir (rather than ls dir)")
	onlyMedia   = flag.Bool("only-media", true, fmt.Sprintf("only list media files (%v)", omxFiletypes))
)

func main() {
	flag.Parse()
	w, err := os.Create("/tmp/plier.log")
	if err != nil {
		log.Panicln(err)
	}
	log.SetOutput(w)
	a := &app{walkDir: *walkDir, player: &player{
		exe: *mediaPlayer,
	}}
	if *useCec {
		go a.initCEC()
		defer a.cec.Destroy()
	}
	a.initCUI()
}
