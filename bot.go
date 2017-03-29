package main

import (
	"fmt"
	"log"

	goricochet "github.com/s-rah/go-ricochet"
)

// TrebuchetBot represents the bot logic.
type TrebuchetBot struct {
	goricochet.StandardRicochetService
	knownContacts  []string
	activeContacts []*TrebuchetConnection
}

// OnNewConnection starts a thread for each new connection.
func (tb *TrebuchetBot) OnNewConnection(oc *goricochet.OpenConnection) {
	if tb.activeContacts == nil {
		tb.activeContacts = make([]*TrebuchetConnection, 0, 1)
		tb.knownContacts = make([]string, 0)
	}
	//if !stringInSlice(tc.Conn.OtherHostname, tc.TrebuchetBot.knownContacts) {
	//	tc.TrebuchetBot.knownContacts = append(tc.TrebuchetBot.knownContacts, tc.Conn.OtherHostname)
	//}
	tb.StandardRicochetService.OnNewConnection(oc)
	tc := &TrebuchetConnection{goricochet.StandardRicochetConnection{}, tb, -1, oc.OtherHostname}

	for i := 0; i < len(tb.activeContacts); i++ {
		tb.activeContacts[i].send(tc.Conn.OtherHostname + " joined the room.")
	}
	tb.activeContacts = append(tb.activeContacts, tc)
	go oc.Process(tc)
}

// TrebuchetConnection represents a connection made by TrebucetBot
type TrebuchetConnection struct {
	goricochet.StandardRicochetConnection
	*TrebuchetBot
	outboundChatChannel int32
	nick                string
}

func (tc *TrebuchetConnection) send(msg string) {
	if tc.outboundChatChannel == -1 {
		cid := int32(6)
		if tc.Conn.Client {
			cid++
		}
		tc.Conn.OpenChatChannel(cid)
		tc.outboundChatChannel = cid
	}
	tc.Conn.SendMessage(tc.outboundChatChannel, msg)
}

// IsKnownContact Always Accepts Contact Requests
func (tc *TrebuchetConnection) IsKnownContact(hostname string) bool {
	return true
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
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
	for i := 0; i < len(tc.activeContacts); i++ {
		if tc.activeContacts[i] != tc {
			tc.activeContacts[i].send(tc.nick + ": " + message)
		}
	}
}

// OnDisconnect fires when the remote connection closes.
func (tc *TrebuchetConnection) OnDisconnect() {
	for i := 0; i < len(tc.TrebuchetBot.activeContacts); i++ {
		if tc.TrebuchetBot.activeContacts[i] == tc {
			copy(tc.TrebuchetBot.activeContacts[i:], tc.TrebuchetBot.activeContacts[i+1:])
			tc.TrebuchetBot.activeContacts[len(tc.TrebuchetBot.activeContacts)-1] = nil
			tc.TrebuchetBot.activeContacts = tc.TrebuchetBot.activeContacts[:len(tc.TrebuchetBot.activeContacts)-1]
			break
		}
	}
	tc.StandardRicochetConnection.OnDisconnect()
}
