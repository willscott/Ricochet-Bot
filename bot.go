package main

import (
	"fmt"
	"log"
	"regexp"

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

	for _, c := range tb.activeContacts {
		c.send(tc.nick + " joined the room.")
	}
	tb.activeContacts = append(tb.activeContacts, tc)
	go oc.Process(tc)
}

// Invite a user to become a contact and start chatting.
func (tb *TrebuchetBot) Invite(addr string) error {
	oc, err := tb.Connect(addr + ".onion:9878")
	if err != nil {
		return err
	}
	tc := &TrebuchetConnection{goricochet.StandardRicochetConnection{}, tb, -1, addr}
	tb.activeContacts = append(tb.activeContacts, tc)

	go oc.Process(tc)
	oc.SendContactRequest(5, "", "You've been invited to join a group chat")
	return nil
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
	re := regexp.MustCompile("^/invite (?:ricochet:)?([a-z0-9]{16})(?:.onion)$")
	if addr := re.FindString(message); len(addr) > 0 {
		if err := tc.TrebuchetBot.Invite(addr); err != nil {
			tc.send("Failed in invite contact: " + err.Error())
		}
		return
	}
	for _, c := range tc.activeContacts {
		if c != tc {
			c.send(tc.nick + ": " + message)
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
	for _, c := range tc.TrebuchetBot.activeContacts {
		c.send(tc.nick + " left the room.")
	}
	tc.StandardRicochetConnection.OnDisconnect()
}
