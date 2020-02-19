package rcom

import (
	"fmt"
	"os"

	"github.com/creack/pty"
)

type port struct {
	*os.File
	filename  string
	isSymlink bool
}

func newPort(filename string, forceLink bool) (p *port, err error) {
	p = &port{filename: filename}

	var fi os.FileInfo
	if fi, err = os.Stat(p.filename); err == nil {
		if mode := fi.Mode(); mode&os.ModeSymlink == os.ModeSymlink {
			if forceLink {
				err = os.Remove(p.filename)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("Symlink %s already exists")
			}
		}
	}

	if _, err = os.Stat(p.filename); err != nil {
		if os.IsNotExist(err) {
			err = nil
			var slave *os.File
			p.File, slave, err = pty.Open()
			if err == nil {
				err = os.Symlink(slave.Name(), p.filename)
				if err == nil {
					p.isSymlink = true
				}
			}
		}
	} else {
		p.File, err = os.OpenFile(p.filename, os.O_RDWR, 0)
	}
	return p, err
}

func (p *port) Close() error {
	err := p.File.Close()
	if err == nil {
		if p.isSymlink {
			err = os.Remove(p.filename)
		}
	}
	return err
}
