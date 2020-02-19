package rcom

import (
	"io"
	"os"
	"sync"
)

func Server(linkname string, forceLink bool) error {
	p, err := newPort(linkname, forceLink)
	if err != nil {
		return err
	}
	defer p.Close()

	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		io.Copy(os.Stdin, p)
		wg.Done()
	}()

	go func() {
		wg.Add(1)
		io.Copy(os.Stdout, p)
		wg.Done()
	}()

	wg.Wait()
	return nil
}
