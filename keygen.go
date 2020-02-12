package rcom

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

type PrivateKey struct {
	*rsa.PrivateKey
}

func NewPrivateKey(bitsize int) (*PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitsize)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return &PrivateKey{privateKey}, nil
}

func (pk *PrivateKey) MarshalPEM() ([]byte, error) {
	// Get ASN.1 DER format
	privDER := x509.MarshalPKCS1PrivateKey(pk.PrivateKey)

	// pem.Block
	privBlock := &pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}

	// Private key in PEM format
	return pem.EncodeToMemory(privBlock), nil
}

func (pk *PrivateKey) PublicKey() (*PublicKey, error) {
	publicKey, err := ssh.NewPublicKey(&pk.PrivateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	return &PublicKey{publicKey}, nil
}

type PublicKey struct {
	ssh.PublicKey
}

func NewPublicKey(buf []byte) (pk *PublicKey, err error) {
	pk = &PublicKey{}
	pk.PublicKey, err = ssh.ParsePublicKey(buf)
	return pk, err
}

func (pk *PublicKey) Decode(reader io.Reader) error {
	lr := bufio.NewReader(reader)
	buf, _, err := lr.ReadLine()
	buf = append(buf, []byte("\n")...)
	if err == nil {
		err = pk.UnmarshalBinary(buf)
	}
	return err
}

func (pk *PublicKey) UnmarshalBinary(buf []byte) error {
	key, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err == nil {
		pk.PublicKey = key
	}
	return err
}

func (pk *PublicKey) MarshalBinary() ([]byte, error) {
	return ssh.MarshalAuthorizedKey(pk.PublicKey), nil
}

func keyConfig(options ...ConfigOption) (*Config, error) {
	config := &Config{
		bitsize: 4096,
	}

	for _, option := range options {
		err := option(config)
		if err != nil {
			return nil, err
		}
	}

	if config.keyfile == "" || config.authorizedKeys == "" {
		u, err := user.Current()
		if err != nil {
			return nil, err
		}
		if config.keyfile == "" {
			config.keyfile = filepath.Join(u.HomeDir, ".ssh", "id_rsa_rcom")
		}

		if config.authorizedKeys == "" {
			config.authorizedKeys = filepath.Join(u.HomeDir, ".ssh", "authorized_keys")
		}
	}
	return config, nil
}

func GenerateKey(options ...ConfigOption) error {
	config, err := keyConfig(options...)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Generating new private key...")
	privateKey, err := NewPrivateKey(config.bitsize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed: %v\n", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "success\n")

	publicKey, err := privateKey.PublicKey()
	if err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Dir(config.keyfile)); os.IsNotExist(err) {
		err = os.Mkdir(filepath.Dir(config.keyfile), 0700)
		if err != nil {
			return fmt.Errorf("Failed to create %s: %v", filepath.Dir(config.keyfile), err)
		}
	} else if err != nil {
		return err
	}

	b, _ := privateKey.MarshalPEM()
	err = ioutil.WriteFile(config.keyfile, b, 0600)
	if err != nil {
		return fmt.Errorf("Failed to write private key %s: %v", config.keyfile, err)
	}

	b, _ = publicKey.MarshalBinary()
	err = ioutil.WriteFile(config.keyfile+".pub", b, 0600)
	if err != nil {
		return fmt.Errorf("Failed to write public key %s.pub: %v", config.keyfile, err)
	}
	return nil
}

func AuthorizeKey(options ...ConfigOption) (err error) {
	config, err := keyConfig(options...)
	if err != nil {
		return err
	}

	var pk PublicKey
	// read the key
	if config.keyfile == "-" {
		// read from stdin
		fmt.Fprintf(os.Stderr, "Reading public key from stdin...")
		err = pk.Decode(os.Stdin)
		if err == nil {
			fmt.Fprintf(os.Stderr, "done\n")
		} else {
			fmt.Fprintf(os.Stderr, "failed: %v", err)
		}
	} else {
		var f *os.File
		if f, err = os.Open(config.keyfile); err == nil {
			err = pk.Decode(f)

			if err != nil && !strings.HasSuffix(config.keyfile, ".pub") {
				if _, err = os.Stat(config.keyfile + ".pub"); err == nil {
					f, err = os.Open(config.keyfile + ".pub")
					if err == nil {
						err = pk.Decode(f)
					}
				} else {
					err = fmt.Errorf("Could not parse public key %v", config.keyfile)
				}
			}
		}
	}

	if err == nil {
		var f *os.File
		if err = os.MkdirAll(filepath.Dir(config.authorizedKeys), 0700); err == nil {
			f, err = os.OpenFile(config.authorizedKeys, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err == nil {
				defer f.Close()
				// TODO add forced command here
				_, err = f.Write(ssh.MarshalAuthorizedKey(pk))
			}
		}
	}
	return err
}

func DeployKey(localDev, hostname, remoteDev string, options ...ConfigOption) error {
	config, err := keyConfig(options...)
	if err != nil {
		return err
	}

	// create key if it doesn't already exist
	_, err = os.Stat(config.keyfile)
	if os.IsNotExist(err) {
		err = GenerateKey(options...)
	}

	if err != nil {
		return err
	}

	publicKey, err := ioutil.ReadFile(config.keyfile)
	if err != nil {
		return err
	}
	publicKey = append(publicKey, []byte("\n")...)
	conn, err := Connect(hostname, append(options, PasswordAuth())...)
	if err != nil {
		return err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = bytes.NewReader(publicKey)
	session.Stderr = os.Stderr
	session.Stdout = os.Stdout

	err = session.Run(fmt.Sprintf("%s key auth -i -", config.exec))
	if err != nil {
		if exerr, ok := err.(*ssh.ExitError); ok {
			if exerr.ExitStatus() == 127 {
				err = fmt.Errorf("%s not found on remote host", config.exec)
			}
		}
	}

	return err
}
