package rcom

import (
	"os"

	"github.com/creack/pty"
	"golang.org/x/crypto/ssh/terminal"
)

type port struct {
	pty      *os.File
	tty      *os.File
	oldState *terminal.State
	linkName string
}

func (p *port) Read(buf []byte) (n int, err error) {
	return p.pty.Read(buf)
}

func (p *port) Write(buf []byte) (n int, err error) {
	return p.pty.Write(buf)
}

func newPort(device string) (p *port, err error) {
	p = &port{}
	if _, err = os.Stat(device); err != nil {
		if os.IsNotExist(err) {
			err = nil
			p.pty, p.tty, err = pty.Open()
			p.oldState, err = terminal.MakeRaw(int(p.pty.Fd()))
			if err != nil {
				Logger.Printf("Failed to activate RAW mode on pty: %v", err)
				return nil, err
			}
			if err == nil {
				err = os.Symlink(p.tty.Name(), device)
				if err == nil {
					p.linkName = device
				} else {
					Logger.Printf("Failed to link %s to %s", p.tty.Name(), device)
				}
			} else {
				Logger.Printf("Failed to open pty")
			}
		}
	} else {
		p.pty, err = os.OpenFile(device, os.O_RDWR, 0)
	}
	return p, err
}

func (p *port) Close() error {
	if p.linkName != "" {
		Logger.Printf("Removing symlink %s", p.linkName)
		os.Remove(p.linkName)
	}
	if p.tty != nil {
		p.tty.Close()
		terminal.Restore(int(p.pty.Fd()), p.oldState)
	}
	return p.pty.Close()
}
