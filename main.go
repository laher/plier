package main

import (
	"flag"
	"log"
	"os"
	"strings"
)

var (
	mediaPlayer      = flag.String("player", "omxplayer", "media player executable for movies etc")
	useCec           = flag.Bool("cec", true, "use cec for HDMI support")
	walkDir          = flag.Bool("walk", true, "walk dir (rather than ls dir)")
	onlyMedia        = flag.Bool("only-media", true, "only list media files (see -media-types)")
	defaultFileTypes = ".mkv,.mp4,.m4v,.avi,mp3"
	mediaTypes       = flag.String("media-types", defaultFileTypes, "file types")
)

func main() {
	flag.Parse()
	w, err := os.Create("/tmp/plier.log")
	if err != nil {
		log.Panicln(err)
	}
	log.SetOutput(w)
	a := &app{walkDir: *walkDir, player: &player{
		exe:       *mediaPlayer,
		filetypes: strings.Split(*mediaTypes, ","),
	}}
	if *useCec {
		go a.initCEC()
		defer a.destroyCEC()
	}
	a.initCUI()
}
