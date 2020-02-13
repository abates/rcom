package rcom

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

func Client(hostname string, devices []string, options ...ConfigOption) error {
	config := &Config{
		exec: "rcom",
	}

	for _, option := range options {
		option(config)
	}

	conn, err := Connect(hostname, options...)
	if err != nil {
		return err
	}
	defer conn.Close()

	var wg sync.WaitGroup
	var errLock sync.Mutex
	var connErr error

	for _, device := range devices {
		localDev, remDev := device, device
		if strings.Contains(device, ":") {
			s := strings.Split(device, ":")
			localDev, remDev = s[0], s[1]
		}
		p, err := newPort(localDev)
		if err != nil {
			Logger.Printf("Failed to attach to port %s: %v", localDev, err)
			return err
		}

		session, err := conn.NewSession()
		if err != nil {
			p.Close()
			return err
		}

		wg.Add(1)
		go func(session *ssh.Session, device string) {
			defer session.Close()
			defer p.Close()
			session.Stdin = p
			session.Stderr = os.Stderr
			session.Stdout = p
			cmd := fmt.Sprintf("%s %q", config.exec, device)
			err = session.Run(cmd)
			if err != nil {
				Logger.Printf("Failed to execute remote command: %q: %v", cmd, err)
				errLock.Lock()
				if connErr == nil {
					connErr = err
				}
				errLock.Unlock()
			}
			wg.Done()
		}(session, remDev)
	}

	wg.Wait()
	return connErr
}
