package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/abates/cli"
	"github.com/abates/rcom"
)

const DefaultExec = "rcom"

var (
	app       *cli.Command
	clientCmd *cli.Command
	deployCmd *cli.Command

	currentUser *user.User

	configDir = ""
	hostname  = ""
	localDev  = ""
	debug     = false

	forceLink      = false
	forceRemote    = false
	username       = ""
	port           = 22
	identity       = ""
	acceptNew      = false
	exec           = DefaultExec
	bitsize        = 4096
	keyfile        = ""
	authorizedKeys = ""
)

func setConnectionFlags(fs *flag.FlagSet) {
	fs.StringVar(&username, "l", currentUser.Username, "login user")
	fs.IntVar(&port, "p", 22, "port to connect on the remote host")
	fs.StringVar(&identity, "i", filepath.Join(currentUser.HomeDir, ".ssh", "id_rsa_"+DefaultExec), "specify identity (private key) file")
	fs.BoolVar(&acceptNew, "a", false, "accept new public keys")
	fs.StringVar(&exec, "e", exec, "executable path/name on remote system")
}

func setKeyFlags(fs *flag.FlagSet) {
	fs.IntVar(&bitsize, "b", 4096, "bitsize")
	fs.StringVar(&keyfile, "f", filepath.Join(currentUser.HomeDir, ".ssh", "id_rsa_"+DefaultExec), "key file")
}

func setDeployFlags(fs *flag.FlagSet) {
	setConnectionFlags(fs)
	setKeyFlags(fs)
}

func init() {
	var err error
	currentUser, err = user.Current()
	if err != nil {
		log.Fatalf("Failed to determine current user: %v", err)
	}
	authorizedKeys = filepath.Join(currentUser.HomeDir, ".ssh", "authorized_keys")

	app = cli.New(
		filepath.Base(os.Args[0]),
		cli.UsageOption("[global options] <command>"),
		cli.CallbackOption(func(string) error {
			if debug {
				rcom.Logger = log.New(os.Stderr, "", log.LstdFlags)
			}
			return nil
		}),
	)
	app.SetOutput(os.Stderr)
	app.Flags.BoolVar(&debug, "debug", false, "turn on debug logging")

	clientCmd = app.SubCommand("client",
		cli.UsageOption("[options] <remote host> <ldev:rdev> [<ldev:rdev> ...]"),
		cli.DescOption("Start client mode"),
		cli.CallbackOption(clientCb),
	)
	setConnectionFlags(&clientCmd.Flags)
	clientCmd.Flags.BoolVar(&forceLink, "f", false, "Force link. Remove link if it exists.")
	clientCmd.Flags.BoolVar(&forceRemote, "fr", false, "Force remote link. Remove remote link if it exists.")
	clientCmd.Arguments.String(&hostname, "remote hostname")

	serverCmd := app.SubCommand("server",
		cli.UsageOption("<local device>"),
		cli.DescOption("Start server mode"),
		cli.CallbackOption(serverCb),
	)
	serverCmd.Flags.BoolVar(&forceLink, "f", false, "Force link. Remove link if it exists.")
	serverCmd.Arguments.String(&localDev, "device path")

	key := app.SubCommand("key",
		cli.UsageOption("<command> [options]"),
		cli.DescOption("Perform ssh public key operations"),
	)

	gen := key.SubCommand("gen", cli.DescOption("Generate a local SSH public/private key pair"), cli.CallbackOption(genCmd))
	setKeyFlags(&gen.Flags)
	auth := key.SubCommand("auth", cli.DescOption("Add a public key to the authorized_keys file"), cli.CallbackOption(authCmd))
	setKeyFlags(&auth.Flags)

	deployCmd = key.SubCommand("deploy",
		cli.UsageOption("[options] <remote host> <rdev> [<rdev> ...]"),
		cli.DescOption("Deploy a public key to a remote host"),
		cli.CallbackOption(deployCb),
	)
	setDeployFlags(&deployCmd.Flags)
	deployCmd.Arguments.String(&hostname, "remote hostname")
}

func main() {
	app.Parse(os.Args[1:])
	err := app.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func clientCb(string) error {
	rcom.Logger.Printf("Connecting to %s", hostname)
	client, err := rcom.Connect(hostname, rcom.Login(username), rcom.Port(port), rcom.IdentityFile(identity), rcom.Accept(acceptNew))
	if err != nil {
		return err
	}

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-ch
		rcom.Logger.Printf("Client received %v", sig)
		client.Close()
		os.Exit(0)
	}()

	for _, device := range clientCmd.Arguments.Args() {
		localDev, remoteDev := device, device
		if strings.Contains(device, ":") {
			s := strings.Split(device, ":")
			localDev, remoteDev = s[0], s[1]
		}

		if strings.HasSuffix(exec, DefaultExec) {
			if debug {
				exec = fmt.Sprintf("%s -debug", exec)
			}

			exec = fmt.Sprintf("%s server", exec)
			if forceRemote {
				exec = fmt.Sprintf("%s -f", exec)
			}
			exec = fmt.Sprintf("%s %s", exec, remoteDev)
		}
		err = client.AttachPTY(localDev, exec, forceLink)
		if err != nil {
			break
		}
	}

	if err == nil {
		client.Wait()
		client.Close()
	}
	return err
}

func serverCb(string) error {
	return rcom.Server(localDev, forceLink)
}

func genCmd(string) error {
	return rcom.GenerateKey(bitsize, keyfile)
}

func authCmd(string) error {
	return rcom.AuthorizeKey(keyfile, authorizedKeys)
}

func deployCb(string) error {
	// create key if it doesn't already exist
	_, err := os.Stat(keyfile)
	if os.IsNotExist(err) {
		err = rcom.GenerateKey(bitsize, keyfile)
	}

	if err != nil {
		return err
	}

	publicKeyfile := keyfile
	if !strings.HasPrefix(publicKeyfile, ".pub") {
		if _, err := os.Stat(publicKeyfile + ".pub"); err == nil {
			publicKeyfile = publicKeyfile + ".pub"
		}
	}

	publicKey, err := ioutil.ReadFile(publicKeyfile)
	if err != nil {
		return err
	}
	publicKey = append(publicKey, []byte("\n")...)

	if strings.HasSuffix(exec, DefaultExec) {
		exec = fmt.Sprintf("%s key auth -f -", exec)
	}
	conn, err := rcom.Connect(hostname, rcom.PasswordAuth(), rcom.Login(username), rcom.Port(port), rcom.Accept(acceptNew))
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Start(exec, bytes.NewReader(publicKey), os.Stdout, os.Stderr)
	if err == nil {
		conn.Wait()
	}
	return err
}
