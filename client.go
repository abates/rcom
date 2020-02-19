package rcom

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

func Client(localDev, hostname, remoteDev string, forceLink bool, options ...ConfigOption) error {
	config := &Config{
		exec: "rcom",
	}

	for _, option := range options {
		option(config)
	}

	p, err := newPort(localDev, forceLink)
	if err != nil {
		return err
	}
	defer p.Close()

	conn, err := Connect(hostname, options...)
	if err != nil {
		return err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = p
	session.Stdout = p
	err = session.Run(fmt.Sprintf("%s %q", config.exec, remoteDev))
	if err != nil {
		if exerr, ok := err.(*ssh.ExitError); ok {
			if exerr.ExitStatus() == 127 {
				err = fmt.Errorf("%s not found on remote host", config.exec)
			}
		}
	}
	return err
}
