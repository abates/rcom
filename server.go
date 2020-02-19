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

	go func() {
		io.Copy(p, os.Stdin)
	}()

	wg.Add(1)
	go func() {
		io.Copy(os.Stdout, p)
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
