package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"

	goricochet "github.com/s-rah/go-ricochet"

	"git.schwanenlied.me/yawning/bulb"
)

var controlPort = flag.String("controlport", "127.0.0.1:9051", "Local socket/path to tor control port.")
var identity = flag.String("identity", "private.key", "Location of ricochet private key identifier.")
var state = flag.String("statefile", "room.state", "Location of ricochet room state file.")

func main() {
	flag.Parse()

	ricochetService := new(TrebuchetBot)

	if _, err := io.Stat(*state); os.IsNotExist(err) {
		fmt.Printf("No State exists. Starting new room.")
	} else {
		bytes, _ := ioutil.ReadFile(*state)
		ricochetService.UnmarshalJSON(bytes)
	}

	if _, err := os.Stat(*identity); os.IsNotExist(err) {
		fmt.Printf("No Private key exists at %s. Creating one.", *identity)
		key, _ := rsa.GenerateKey(rand.Reader, 1024)
		dat := x509.MarshalPKCS1PrivateKey(key)
		block := pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   dat,
		}
		x := pem.EncodeToMemory(&block)
		ioutil.WriteFile(*identity, x, 0600)
	}
	if err := ricochetService.Init(*identity); err != nil {
		panic(err)
	}

	// Make the listener for the ricochet client
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	_, localPort, _ := net.SplitHostPort(ln.Addr().String())

	// read in the privaate key we're usng and give it to tor
	pemData, err := ioutil.ReadFile(*identity)
	if err != nil {
		panic("Failed to read: " + err.Error())
	}
	block, _ := pem.Decode(pemData)
	pki, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

	ports := make([]bulb.OnionPortSpec, 1)
	ports[0].VirtPort = 9878
	ports[0].Target = localPort
	net := "tcp4"
	if (*controlPort)[0] == '/' {
		net = "unix"
	}
	c, err := bulb.Dial(net, *controlPort)
	if err != nil {
		panic("Could not connect to tor: " + err.Error())
	}
	err = c.Authenticate("")
	if err != nil {
		panic("Could not authenticate with tor: " + err.Error())
	}
	inf, err := c.AddOnion(ports, pki, false)
	if err != nil {
		panic(err)
	}

	// register for signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		data, _ := ricochetService.MarshalJSON()
		ioutil.WriteFile(*state, data, 600)
		os.Exit(0)
	}()

	fmt.Printf("Listening at: %s ", inf.OnionID)

	goricochet.Serve(ln, ricochetService)
}
