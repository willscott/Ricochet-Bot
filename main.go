package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	goricochet "github.com/s-rah/go-ricochet"

	"git.schwanenlied.me/yawning/bulb"
)

// TrebuchetBot represents the bot logic.
type TrebuchetBot struct {
	goricochet.StandardRicochetService
}

// IsKnownContact Always Accepts Contact Requests
func (tb *TrebuchetBot) IsKnownContact(hostname string) bool {
	return true
}

// OnContactRequest fires on contact requests
func (tb *TrebuchetBot) OnContactRequest(oc *goricochet.OpenConnection, channelID int32, nick string, message string) {
	//tb.StandardRicochetService.OnNewConnection(oc)
	//tb.StandardRicochetService.OnContactRequest(channelID, nick, message)
	oc.AckContactRequestOnResponse(channelID, "Accepted")
	oc.CloseChannel(channelID)
}

// OnChatMessage fires on incoming chat mressages.
func (tb *TrebuchetBot) OnChatMessage(oc *goricochet.OpenConnection, channelID int32, messageID int32, message string) {
	log.Printf("Received Message from %s: %s", oc.OtherHostname, message)
	oc.AckChatMessage(channelID, messageID)
	if oc.GetChannelType(6) == "none" {
		oc.OpenChatChannel(6)
	}
	oc.SendMessage(6, message)
}

var controlPort = flag.String("controlport", "127.0.0.1:9051", "Local socket/path to tor control port.")

func main() {
	flag.Parse()

	ricochetService := new(TrebuchetBot)
	if _, err := os.Stat("./private_key"); os.IsNotExist(err) {
		key, _ := rsa.GenerateKey(rand.Reader, 1024)
		dat := x509.MarshalPKCS1PrivateKey(key)
		block := pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   dat,
		}
		x := pem.EncodeToMemory(&block)
		ioutil.WriteFile("./private_key", x, 0600)
	}
	if err := ricochetService.Init("./private_key"); err != nil {
		panic(err)
	}

	// separate from giving ricohet the private key. also give it to tor for an onion:
	pemData, err := ioutil.ReadFile("./private_key")
	if err != nil {
		panic("Failed to read: " + err.Error())
	}
	block, _ := pem.Decode(pemData)
	pki, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

	ports := make([]bulb.OnionPortSpec, 1)
	ports[0].VirtPort = 9878
	ports[0].Target = "12345"
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

	fmt.Printf("Listening at: %s ", inf.OnionID)

	ricochetService.Listen(ricochetService, 12345)
}
