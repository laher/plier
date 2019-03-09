package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/laher/cec"
)

type app struct {
	pwd     string
	walkDir bool
	isCEC   bool
	player  *player
	g       *gocui.Gui

	lines []string
	cec   *cec.Connection
}

const maxDirs = 50

func (a *app) walkMain(v io.Writer) error {
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
				for _, ft := range a.player.filetypes {
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

func (a *app) lsMain(v io.Writer) error {
	files, err := ioutil.ReadDir(a.pwd)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			fmt.Fprintln(v, f.Name())
		}
	}
	return nil
}
