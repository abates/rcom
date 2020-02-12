package rcom

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
)

type Connection struct {
	*ssh.Client
	config *Config
}

func (conn *Connection) hostKeyCallback(hostname string, remote net.Addr, key ssh.PublicKey) error {
	fields := strings.Split(hostname, ":")
	if fields[1] == "22" {
		hostname = fields[0]
	} else {
		hostname = fmt.Sprintf("[%s]:%s", fields[0], fields[1])
	}

	remoteAddr := remote.String()
	fields = strings.Split(remoteAddr, ":")
	if fields[1] == "22" {
		remoteAddr = fields[0]
	} else {
		remoteAddr = fmt.Sprintf("[%s]:%s", fields[0], fields[1])
	}

	if _, err := os.Stat(conn.config.knownHosts); err != nil {
		if os.IsNotExist(err) {
			if conn.config.acceptNew {
				if err := os.MkdirAll(filepath.Dir(conn.config.knownHosts), 0700); err != nil {
					return err
				}

				line := kh.Line([]string{hostname}, key)
				return ioutil.WriteFile(conn.config.knownHosts, append([]byte(line), []byte("\n")...), 0600)
			} else {
				return fmt.Errorf("Could not verify %s: known_hosts file not found", hostname)
			}
		} else {
			return err
		}
	}

	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		_, hosts, pk, _, _, err := ssh.ParseKnownHosts(scanner.Bytes())
		if err != nil {
			return err
		}
		for _, host := range hosts {
			if host == hostname || host == remoteAddr {
				err = ssh.FixedHostKey(pk)(hostname, remote, key)
				return err
			}
		}
	}

	if conn.config.acceptNew {
		line := kh.Line([]string{hostname}, key)
		f, err := os.OpenFile(conn.config.knownHosts, os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			_, err = f.WriteString(line + "\n")
		}
		f.Close()
		return err
	}
	return fmt.Errorf("Hostkey verification failed, no entry in known_hosts")
}

func Connect(hostname string, options ...ConfigOption) (*Connection, error) {
	config := &Config{
		port: 22,
	}

	for _, option := range options {
		option(config)
	}

	if config.identityAuth == nil || config.clientConfig.User == "" || config.knownHosts == "" {
		u, err := user.Current()
		if err != nil {
			return nil, err
		}

		if config.identityAuth == nil {
			err := DefaultIdentityFile(u.HomeDir)(config)
			if err != nil {
				return nil, err
			}
		}

		if config.identityAuth != nil {
			config.clientConfig.Auth = append(config.clientConfig.Auth, config.identityAuth)
		}

		if config.passwordAuth != nil {
			config.clientConfig.Auth = append(config.clientConfig.Auth, config.passwordAuth)
		}

		if config.clientConfig.User == "" {
			config.clientConfig.User = u.Username
		}

		if config.knownHosts == "" {
			config.knownHosts = filepath.Join(u.HomeDir, ".ssh", "known_hosts")
		}
	}

	conn := &Connection{config: config}
	config.clientConfig.HostKeyCallback = conn.hostKeyCallback

	var err error
	conn.Client, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", hostname, config.port), &config.clientConfig)
	return conn, err
}
