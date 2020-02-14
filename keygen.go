package rcom

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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

func GenerateKey(bitsize int, keyfile string) error {
	Logger.Printf("Generating new private key...")
	privateKey, err := NewPrivateKey(bitsize)
	if err != nil {
		Logger.Printf("failed: %v\n", err)
		return err
	}
	Logger.Printf("success\n")

	publicKey, err := privateKey.PublicKey()
	if err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Dir(keyfile)); os.IsNotExist(err) {
		err = os.Mkdir(filepath.Dir(keyfile), 0700)
		if err != nil {
			return fmt.Errorf("Failed to create %s: %v", filepath.Dir(keyfile), err)
		}
	} else if err != nil {
		return err
	}

	b, _ := privateKey.MarshalPEM()
	err = ioutil.WriteFile(keyfile, b, 0600)
	if err != nil {
		return fmt.Errorf("Failed to write private key %s: %v", keyfile, err)
	}

	b, _ = publicKey.MarshalBinary()
	err = ioutil.WriteFile(keyfile+".pub", b, 0600)
	if err != nil {
		return fmt.Errorf("Failed to write public key %s.pub: %v", keyfile, err)
	}
	return nil
}

func AuthorizeKey(keyfile, authorizedKeys string) (err error) {
	var pk PublicKey
	// read the key
	if keyfile == "-" {
		// read from stdin
		err = pk.Decode(os.Stdin)
	} else {
		var f *os.File
		if f, err = os.Open(keyfile); err == nil {
			err = pk.Decode(f)

			if err != nil && !strings.HasSuffix(keyfile, ".pub") {
				if _, err = os.Stat(keyfile + ".pub"); err == nil {
					f, err = os.Open(keyfile + ".pub")
					if err == nil {
						err = pk.Decode(f)
					}
				} else {
					Logger.Printf("Failed to parse public key %s: %v", keyfile, err)
				}
			}
		}
	}

	if err == nil {
		var f *os.File
		if err = os.MkdirAll(filepath.Dir(authorizedKeys), 0700); err == nil {
			f, err = os.OpenFile(authorizedKeys, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err == nil {
				defer f.Close()
				// TODO add forced command here
				_, err = f.Write(ssh.MarshalAuthorizedKey(pk))
			}
		} else {
			Logger.Printf("Failed to create %s: %v", filepath.Dir(authorizedKeys))
		}
	}
	return err
}
