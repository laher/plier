package main

import (
	"errors"
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
	if a.cmd != nil {
		err = a.stop()
		if err != nil {
			return err
		}
	}
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
	if a.writer == nil {
		return errNotPlaying
	}
	_, err := a.writer.Write([]byte("up"))
	return err
}

func (a *player) rewind() error {
	if a.writer == nil {
		return errNotPlaying
	}
	_, err := a.writer.Write([]byte("dn"))
	return err
}

var errNotPlaying = errors.New("Not playing")

func (a *player) play() error {
	if a.writer == nil {
		return errNotPlaying
	}
	_, err := a.writer.Write([]byte("p"))
	return err
}

func (a *player) pause() error {
	if a.writer == nil {
		return errNotPlaying
	}
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
