package rcom

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	Logger.Printf("Connecting server to %s", linkname)
	p, err := newPort(linkname)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		_, err := io.Copy(p, os.Stdin)
		if err != nil {
			Logger.Printf("Failed to copy from Stdin: %v", err)
		}
		Logger.Printf("Stdin done")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		_, err = io.Copy(os.Stdout, p)
		Logger.Printf("Stdout done")
		wg.Done()
	}()

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	closed := false
	go func() {
		<-ch
		p.CloseTTY()
		closed = true
	}()
	wg.Wait()
	if !closed {
		p.CloseTTY()
	}
	return nil
}
