package rcom

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
)

type Connection struct {
	*ssh.Client
	config   *Config
	sessions []*ssh.Session
	wg       sync.WaitGroup
}

/*
func (conn *Connection) Run(exec string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	session, err := conn.Start(exec, stdin, stdout, stderr)
	if err == nil {
		err = session.Wait()
	}
	return err
}*/

type logWriter struct {
	io.Writer
}

func (lw logWriter) Write(p []byte) (int, error) {
	log.Printf("Write: %s", string(p))
	return lw.Writer.Write(p)
}

type logReader struct {
	io.Reader
}

func (lr logReader) Read(p []byte) (n int, err error) {
	n, err = lr.Reader.Read(p)
	log.Printf(" Read: %s", string(p[0:n]))
	return
}

func (conn *Connection) Start(exec string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (*ssh.Session, error) {
	session, err := conn.NewSession()
	if err != nil {
		Logger.Printf("Failed to create ssh session: %v", err)
		return nil, err
	}

	//session.Stdin = stdin
	session.Stdin = logReader{stdin}
	//session.Stdout = stdout
	session.Stdout = logWriter{stdout}
	session.Stderr = stderr
	err = session.Start(exec)
	if err == nil {
		conn.wg.Add(1)
		go func() {
			session.Wait()
			conn.wg.Done()
		}()
	} else {
		Logger.Printf("Failed to start remote command: %q: %v", exec, err)
	}
	return session, err
}

func (conn *Connection) AttachPTY(localDev string, exec string, force bool) error {
	Logger.Printf("Attaching to local port %s", localDev)
	p, err := newPort(localDev, force)
	if err != nil {
		Logger.Printf("Failed to attach to port %s: %v", localDev, err)
		p.ClosePTY()
		return err
	}

	Logger.Printf("Executing %q on remote host", exec)
	session, err := conn.Start(exec, p, p, os.Stderr)
	if err == nil {
		conn.sessions = append(conn.sessions, session)
	} else {
		p.ClosePTY()
	}

	return err
}

func (conn *Connection) Wait() {
	conn.wg.Wait()
}

func (conn *Connection) Close() error {
	for _, session := range conn.sessions {
		session.Signal(ssh.SIGINT)
		if p, ok := session.Stdin.(*port); ok {
			p.ClosePTY()
		}
		session.Close()
	}
	conn.sessions = nil
	return nil
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
		port:      22,
		keepAlive: time.Second * 60,
		clientConfig: ssh.ClientConfig{
			Timeout: time.Second * 5,
		},
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
	if err == nil && config.keepAlive > 0 {
		go func() {
			for {
				<-time.After(config.keepAlive)
				println("Sending keep alive")
				_, _, err := conn.Client.SendRequest("rcom keep alive", true, nil)
				if err != nil {
					conn.Close()
					return
				}
			}
		}()
	}
	return conn, err
}
