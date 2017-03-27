package main

import (
	"fmt"
	"log"

	goricochet "github.com/s-rah/go-ricochet"
)

// TrebuchetBot represents the bot logic.
type TrebuchetBot struct {
	goricochet.StandardRicochetService
}

// OnNewConnection starts a thread for each new connection.
func (tb *TrebuchetBot) OnNewConnection(oc *goricochet.OpenConnection) {
	tb.StandardRicochetService.OnNewConnection(oc)
	go oc.Process(&TrebuchetConnection{})
}

// TrebuchetConnection represents a connection made by TrebucetBot
type TrebuchetConnection struct {
	goricochet.StandardRicochetConnection
}

// IsKnownContact Always Accepts Contact Requests
func (tc *TrebuchetConnection) IsKnownContact(hostname string) bool {
	return true
}

// OnContactRequest fires on contact requests
func (tc *TrebuchetConnection) OnContactRequest(channelID int32, nick string, message string) {
	tc.StandardRicochetConnection.OnContactRequest(channelID, nick, message)
	fmt.Printf("Contact request from %s: %s", nick, message)
	tc.Conn.AckContactRequestOnResponse(channelID, "Accepted")
	tc.Conn.CloseChannel(channelID)
}

// OnChatMessage fires on incoming chat mressages.
func (tc *TrebuchetConnection) OnChatMessage(channelID int32, messageID int32, message string) {
	log.Printf("Received Message from %s: %s", tc.Conn.OtherHostname, message)
	tc.Conn.AckChatMessage(channelID, messageID)
	if tc.Conn.GetChannelType(6) == "none" {
		tc.Conn.OpenChatChannel(6)
	}
	tc.Conn.SendMessage(6, message)
}
