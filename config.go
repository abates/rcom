package rcom

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

var Logger = log.New(ioutil.Discard, "", 0)

type Config struct {
	acceptNew      bool // acceptNew entries to known_hosts
	port           int
	exec           string
	bitsize        int
	keyfile        string
	knownHosts     string
	authorizedKeys string

	identityAuth ssh.AuthMethod
	passwordAuth ssh.AuthMethod
	clientConfig ssh.ClientConfig
}

type ConfigOption func(*Config) error

func Login(username string) ConfigOption {
	return func(config *Config) error {
		config.clientConfig.User = username
		return nil
	}
}

func Port(port int) ConfigOption {
	return func(config *Config) error {
		if port < 1 || 65535 < port {
			return fmt.Errorf("Setting destination port failed: Valid port values are 1-65535")
		}
		config.port = port
		return nil
	}
}

func Accept(accept bool) ConfigOption {
	return func(config *Config) error {
		config.acceptNew = accept
		return nil
	}
}

func Exec(exec string) ConfigOption {
	return func(config *Config) (err error) {
		// we don't check if exec exists here. Although it might not exist
		// locally, it might exist remotely
		config.exec = exec
		return nil
	}
}

func BitSize(size int) ConfigOption {
	return func(config *Config) error {
		config.bitsize = size
		return nil
	}
}

func KeyFile(file string) ConfigOption {
	return func(config *Config) error {
		config.keyfile = file
		return nil
	}
}

func KnownHosts(file string) ConfigOption {
	return func(config *Config) error {
		config.knownHosts = file
		return nil
	}
}

func AuthorizedKeys(file string) ConfigOption {
	return func(config *Config) error {
		config.authorizedKeys = file
		return nil
	}
}

func IdentityFile(file string) ConfigOption {
	return func(config *Config) error {
		if file == "" {
			return nil
		}

		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("No such identity file: %s", file)
		} else if err != nil {
			return err
		}

		key, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("Failed to read identity file %s: %v", file, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("Failed to parse private key %s: %v", file, err)
		}
		config.identityAuth = ssh.PublicKeys(signer)
		return nil
	}
}

func DefaultIdentityFile(homedir string) ConfigOption {
	return func(config *Config) error {
		for _, f := range []string{"id_dsa_rcom", "id_ecdsa_rcom", "id_ed25519_rcom", "id_rsa_rcom"} {
			f = filepath.Join(homedir, ".ssh", f)
			if _, err := os.Stat(f); err == nil {
				return IdentityFile(f)(config)
			}
		}
		return fmt.Errorf("No identify file found in %s", homedir)
	}
}

type readWriter struct {
	io.Reader
	io.Writer
}

func PasswordAuth() ConfigOption {
	return func(config *Config) error {
		config.passwordAuth = ssh.PasswordCallback(func() (string, error) {
			fmt.Fprintf(os.Stdout, "Password: ")
			pass, err := terminal.ReadPassword(0)
			fmt.Fprintf(os.Stdout, "\n")
			return string(pass), err
		})
		return nil
	}
}
