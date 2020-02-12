package rcom

import (
	"io"
	"os"
	"sync"
)

func Server(linkname string) error {
	p := &port{filename: linkname}
	err := p.setup()
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
