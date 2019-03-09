package main

import (
	"io"
	"os/exec"
	"strings"
)

type player struct {
	exe string

	writer    io.Writer
	cmd       *exec.Cmd
	filetypes []string
}

func (a *player) supports(item string) bool {

	for _, ft := range a.filetypes {
		if strings.HasSuffix(item, ft) {
			return true
		}
	}
	return false
}
func (a *player) start(item string) error {
	var err error
	a.cmd = exec.Command(a.exe, item)
	a.writer, err = a.cmd.StdinPipe()
	if err != nil {
		return err
	}

	err = a.cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

func (a *player) ff() error {
	_, err := a.writer.Write([]byte("up"))
	return err
}

func (a *player) rewind() error {
	_, err := a.writer.Write([]byte("dn"))
	return err
}

func (a *player) play() error {
	_, err := a.writer.Write([]byte("p"))
	return err
}

func (a *player) pause() error {
	_, err := a.writer.Write([]byte("p"))
	return err
}

func (a *player) stop() error {
	var err error
	if a.cmd != nil {
		_, err = a.writer.Write([]byte("q"))
		a.cmd.Process.Kill()
		a.cmd = nil
		a.writer = nil
	}
	return err
}
