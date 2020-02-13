package rcom

import (
	"io"
	"os"
	"sync"
)

type reader struct {
	upstream io.Reader
}

func (r reader) Read(p []byte) (n int, err error) {
	n, err = r.upstream.Read(p)
	if err == nil {
		Logger.Printf("Read %d bytes: %x", n, p[0:n])
	}
	return n, err
}

func Server(linkname string) error {
	p, err := newPort(linkname)
	if err != nil {
		return err
	}
	defer p.Close()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		_, err := io.Copy(p, os.Stdin)
		if err != nil {
			Logger.Printf("Failed to copy from Stdin: %v", err)
		}
		wg.Done()
	}()

	_, err = io.Copy(os.Stdout, p)
	if err != nil {
		Logger.Printf("Failed to copy to Stdout: %v", err)
	}

	wg.Wait()
	return nil
}
