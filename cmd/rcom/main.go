package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/abates/cli"
	"github.com/abates/rcom"
)

var (
	app    *cli.Command
	client *cli.Command
	deploy *cli.Command

	currentUser *user.User

	configDir = ""
	hostname  = ""
	localDev  = ""
	debug     = false

	username  = ""
	port      = 22
	identity  = ""
	acceptNew = false
	exec      = "rcom"
	bitsize   = 4096
	keyfile   = ""
)

func setConnectionFlags(fs *flag.FlagSet) {
	fs.StringVar(&username, "l", currentUser.Username, "login user")
	fs.IntVar(&port, "p", 22, "port to connect on the remote host")
	fs.StringVar(&identity, "i", "", "specify identity (private key) file")
	fs.BoolVar(&acceptNew, "a", false, "accept new public keys")
	fs.StringVar(&exec, "e", "rcom", "executable path/name on remote system")
}

func setKeyFlags(fs *flag.FlagSet) {
	fs.IntVar(&bitsize, "b", 4096, "bitsize")
	fs.StringVar(&keyfile, "f", filepath.Join(currentUser.HomeDir, ".ssh", "id_rsa_rcom"), "key file")
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

	client = app.SubCommand("client",
		cli.UsageOption("[options] <remote host> <ldev:rdev> [<ldev:rdev> ...]"),
		cli.DescOption("Start client mode"),
		cli.CallbackOption(clientCmd),
	)
	setConnectionFlags(&client.Flags)
	client.Arguments.String(&hostname, "remote hostname")

	server := app.SubCommand("server",
		cli.UsageOption("<local device>"),
		cli.DescOption("Start server mode"),
		cli.CallbackOption(serverCmd),
	)
	server.Arguments.String(&localDev, "device path")

	key := app.SubCommand("key",
		cli.UsageOption("<command> [options]"),
		cli.DescOption("Perform ssh public key operations"),
	)

	gen := key.SubCommand("gen", cli.DescOption("Generate a local SSH public/private key pair"), cli.CallbackOption(genCmd))
	setKeyFlags(&gen.Flags)
	auth := key.SubCommand("auth", cli.DescOption("Add a public key to the authorized_keys file"), cli.CallbackOption(authCmd))
	setKeyFlags(&auth.Flags)

	deploy = key.SubCommand("deploy",
		cli.UsageOption("[options] <remote host> <rdev> [<rdev> ...]"),
		cli.DescOption("Deploy a public key to a remote host"),
		cli.CallbackOption(deployCmd),
	)
	setDeployFlags(&deploy.Flags)
	deploy.Arguments.String(&hostname, "remote hostname")
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

func clientCmd(string) error {
	return rcom.Client(hostname, client.Arguments.Args(), rcom.Login(username), rcom.Port(port), rcom.IdentityFile(identity), rcom.Accept(acceptNew), rcom.Exec(exec))
}

func serverCmd(string) error {
	return rcom.Server(localDev)
}

func genCmd(string) error {
	return rcom.GenerateKey(rcom.KeyFile(keyfile), rcom.BitSize(bitsize))
}

func authCmd(string) error {
	return rcom.AuthorizeKey(rcom.KeyFile(keyfile), rcom.BitSize(bitsize))
}

func deployCmd(string) error {
	return rcom.DeployKey(hostname, deploy.Arguments.Args(), rcom.KeyFile(keyfile), rcom.BitSize(bitsize), rcom.Login(username), rcom.Port(port), rcom.IdentityFile(identity), rcom.Accept(acceptNew), rcom.Exec(exec))
}
