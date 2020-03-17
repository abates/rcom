package rcom

import (
	"io"
	"os"
	"os/signal"
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

func Server(linkname string, force bool) error {
	Logger.Printf("Connecting server to %s", linkname)
	p, err := newPort(linkname, force)
	if err != nil {
		return err
	}

	done := make(chan interface{}, 3)
	go func() {
		io.Copy(p, os.Stdin)
		done <- true
	}()

	go func() {
		io.Copy(os.Stdout, p)
		done <- true
	}()

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	closed := false
	go func() {
		sig := <-ch
		Logger.Printf("Server received %v", sig)
		p.CloseTTY()
		closed = true
		done <- true
	}()
	<-done
	if !closed {
		p.CloseTTY()
	}
	return nil
}
