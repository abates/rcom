package rcom

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/crypto/ssh"
)

type client struct {
	wg       sync.WaitGroup
	errLock  sync.Mutex
	connErr  error
	sessions []*ssh.Session
	config   Config
}

func (c *client) cleanupSessions() {
	for _, session := range c.sessions {
		if p, ok := session.Stdin.(*port); ok {
			p.ClosePTY()
		}
		session.Signal(ssh.SIGINT)
		session.Close()
	}
	c.sessions = nil
}

func (c *client) handleCtrlC() {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT)
	go func() {
		<-ch
		c.cleanupSessions()
	}()
}

func (c *client) portHandler(session *ssh.Session, p *port, device string) {
	//defer session.Close()
	session.Stdin = p
	session.Stderr = os.Stderr
	session.Stdout = p
	cmd := fmt.Sprintf("%s %q", c.config.exec, device)
	err := session.Run(cmd)
	if err != nil {
		Logger.Printf("Failed to execute remote command: %q: %v", cmd, err)
		c.errLock.Lock()
		if c.connErr == nil {
			c.connErr = err
		}
		c.errLock.Unlock()
	}
	c.wg.Done()
}

func Client(hostname string, devices []string, options ...ConfigOption) error {
	c := &client{
		config: Config{exec: "rcom"},
	}

	for _, option := range options {
		option(&c.config)
	}

	conn, err := Connect(hostname, options...)
	if err != nil {
		return err
	}

	for _, device := range devices {
		localDev, remDev := device, device
		if strings.Contains(device, ":") {
			s := strings.Split(device, ":")
			localDev, remDev = s[0], s[1]
		}
		p, err := newPort(localDev)
		if err != nil {
			c.cleanupSessions()
			Logger.Printf("Failed to attach to port %s: %v", localDev, err)
			return err
		}

		session, err := conn.NewSession()
		if err != nil {
			p.ClosePTY()
			c.cleanupSessions()
			return err
		}
		c.sessions = append(c.sessions, session)
		c.wg.Add(1)
		go c.portHandler(session, p, remDev)
	}

	c.handleCtrlC()

	c.wg.Wait()
	c.cleanupSessions()
	conn.Close()

	return c.connErr
}
