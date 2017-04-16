package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"unicode"
	"unicode/utf8"

	goricochet "github.com/s-rah/go-ricochet"
)

// TrebuchetContact represents saved information about a contact of the bot
type TrebuchetContact struct {
	Hostname string
	Nickname string
	Pending  []string
}

// TrebuchetBot represents the bot logic.
type TrebuchetBot struct {
	goricochet.StandardRicochetService
	knownContacts  []TrebuchetContact
	activeContacts []*TrebuchetConnection

	// Settings
	AllowConnections      bool
	AllowInvites          bool
	GeneratedNicks        bool
	JoinPartNotifications bool
	ReplayLogs            bool
}

// OnNewConnection starts a thread for each new connection.
func (tb *TrebuchetBot) OnNewConnection(oc *goricochet.OpenConnection) {
	if tb.activeContacts == nil {
		tb.activeContacts = make([]*TrebuchetConnection, 0, 1)
		tb.knownContacts = make([]TrebuchetContact, 0)
	}
	//if !stringInSlice(tc.Conn.OtherHostname, tc.TrebuchetBot.knownContacts) {
	//	tc.TrebuchetBot.knownContacts = append(tc.TrebuchetBot.knownContacts, tc.Conn.OtherHostname)
	//}
	tb.StandardRicochetService.OnNewConnection(oc)
	tc := &TrebuchetConnection{goricochet.StandardRicochetConnection{}, tb, -1, oc.OtherHostname, false}

	tb.activeContacts = append(tb.activeContacts, tc)
	go oc.Process(tc)
}

// Invite a user to become a contact and start chatting.
func (tb *TrebuchetBot) Invite(addr string, nick string) error {
	oc, err := tb.Connect(addr)
	if err != nil {
		return err
	}
	tc := &TrebuchetConnection{goricochet.StandardRicochetConnection{
		Conn:       oc,
		PrivateKey: tb.PrivateKey,
	}, tb, -1, nick, true}
	tb.activeContacts = append(tb.activeContacts, tc)

	known := false
	for _, k := range tb.knownContacts {
		if k.Hostname == addr {
			known = true
			break
		}
	}
	if !known {
		tb.knownContacts = append(tb.knownContacts, TrebuchetContact{
			Hostname: addr,
			Nickname: nick,
			Pending:  make([]string, 0),
		})
	}

	go oc.Process(tc)
	return nil
}

// UnmarshalJSON restores the bot from state
func (tb *TrebuchetBot) UnmarshalJSON(data []byte) error {
	known := make([]TrebuchetContact, 0)
	if err := json.Unmarshal(data, known); err != nil {
		return err
	}
	params := make([]bool, 5)
	json.Unmarshal([]byte(known[0].Nickname), params)
	tb.AllowConnections = params[0]
	tb.AllowInvites = params[1]
	tb.GeneratedNicks = params[2]
	tb.JoinPartNotifications = params[3]
	tb.ReplayLogs = params[4]

	tb.knownContacts = known[1:]
	for _, k := range tb.knownContacts {
		tb.Invite(k.Hostname, k.Nickname)
	}
	return nil
}

// MarshalJSON produces a string encoding of bot state.
func (tb *TrebuchetBot) MarshalJSON() ([]byte, error) {
	params := []bool{tb.AllowConnections, tb.AllowInvites, tb.GeneratedNicks, tb.JoinPartNotifications, tb.ReplayLogs}
	marshaledParams, _ := json.Marshal(params)
	mush := append([]TrebuchetContact{TrebuchetContact{Hostname: "", Nickname: string(marshaledParams), Pending: nil}}, tb.knownContacts...)
	return json.Marshal(mush)
}

// TrebuchetConnection represents a connection made by TrebucetBot
type TrebuchetConnection struct {
	goricochet.StandardRicochetConnection
	*TrebuchetBot
	outboundChatChannel int32
	nick                string
	TriggerInvite       bool
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
	if tc.TrebuchetBot.AllowConnections {
		return true
	}
	for _, k := range tc.TrebuchetBot.knownContacts {
		if k.Hostname == hostname {
			return true
		}
	}
	return false
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// OnReady triggers when the connection is opened
func (tc *TrebuchetConnection) OnReady(oc *goricochet.OpenConnection) {
	tc.StandardRicochetConnection.OnReady(oc)
	if tc.TriggerInvite {
		oc.SendContactRequest(5, tc.nick, "You've been invited to join a group chat")
	}
}

// OnContactRequest fires on contact requests
func (tc *TrebuchetConnection) OnContactRequest(channelID int32, nick string, message string) {
	tc.StandardRicochetConnection.OnContactRequest(channelID, nick, message)
	fmt.Printf("Contact request from %s: %s", nick, message)
	tc.Conn.AckContactRequestOnResponse(channelID, "Accepted")
	tc.Conn.CloseChannel(channelID)
}

// OnAuthenticationProof fires when the remote user authenticates itself.
func (tc *TrebuchetConnection) OnAuthenticationProof(channelID int32, pubkey []byte, sig []byte) {
	tc.StandardRicochetConnection.OnAuthenticationProof(channelID, pubkey, sig)

	if len(tc.nick) == 0 {
		if tc.TrebuchetBot.GeneratedNicks {
			tc.nick = "anonymous"
		} else {
			for _, k := range tc.TrebuchetBot.knownContacts {
				if k.Hostname == tc.Conn.OtherHostname {
					tc.nick = k.Nickname
				}
			}
			if len(tc.nick) == 0 {
				tc.nick = tc.Conn.OtherHostname
			}
		}
	}

	log.Printf("Connection from %s (%s)\n", tc.Conn.OtherHostname, tc.nick)
	if tc.TrebuchetBot.JoinPartNotifications {
		for _, c := range tc.TrebuchetBot.activeContacts {
			if c != tc {
				c.send(tc.nick + " joined the room.")
			}
		}
	}

	known := false
	for _, k := range tc.TrebuchetBot.knownContacts {
		if k.Hostname == tc.Conn.OtherHostname {
			known = true
			if len(k.Pending) > 0 {
				for _, p := range k.Pending {
					tc.send(p)
				}
				k.Pending = make([]string, 0)
			}
		}
	}
	if !known {
		tc.TrebuchetBot.knownContacts = append(tc.TrebuchetBot.knownContacts, TrebuchetContact{
			Hostname: tc.Conn.OtherHostname,
			Nickname: tc.nick,
			Pending:  make([]string, 0),
		})
	}
}

var invitere = regexp.MustCompile(`^\/invite (?:ricochet:)?([a-z0-9]{16})(?:\.onion)?\s?(.*)$`)
var partre = regexp.MustCompile(`^\/part$`)
var nickre = regexp.MustCompile(`^\/nick (.*)$`)

// OnChatMessage fires on incoming chat mressages.
func (tc *TrebuchetConnection) OnChatMessage(channelID int32, messageID int32, message string) {
	log.Printf("Received Message from %s: %s", tc.Conn.OtherHostname, message)
	tc.Conn.AckChatMessage(channelID, messageID)

	//fmt.Printf("invite re: %v\n", invitere.FindStringSubmatch(message))
	if addr := invitere.FindStringSubmatch(message); len(addr) > 1 && len(addr[1]) > 0 {
		if !tc.TrebuchetBot.AllowInvites {
			return
		}
		nick := ""
		tc.send("Inviting " + addr[1])
		if len(addr) > 2 && validNickname(addr[2]) {
			nick = addr[2]
		}
		if err := tc.TrebuchetBot.Invite(addr[1], nick); err != nil {
			tc.send("Failed in invite contact: " + err.Error())
		}
		return
	} else if partre.MatchString(message) {
		tc.Conn.Close()
		for i, k := range tc.TrebuchetBot.knownContacts {
			if k.Hostname == tc.Conn.OtherHostname {
				tc.TrebuchetBot.knownContacts[i] = tc.TrebuchetBot.knownContacts[len(tc.TrebuchetBot.knownContacts)-1]
				tc.TrebuchetBot.knownContacts = tc.TrebuchetBot.knownContacts[0 : len(tc.TrebuchetBot.knownContacts)-1]
			}
		}
	}
	if tc.TrebuchetBot.ReplayLogs {
		for _, c := range tc.TrebuchetBot.knownContacts {
			found := false
			for _, a := range tc.activeContacts {
				if a.Conn.OtherHostname == c.Hostname {
					a.send(tc.nick + ": " + message)
					found = true
					break
				}
			}
			if !found {
				c.Pending = append(c.Pending, tc.nick+": "+message)
			}
		}
	} else {
		for _, c := range tc.activeContacts {
			if c != tc {
				c.send(tc.nick + ": " + message)
			}
		}
	}
	//fmt.Printf("nick re: %v\n", nickre.FindStringSubmatch(message))
	if name := nickre.FindStringSubmatch(message); len(name) > 1 && len(name[1]) > 0 && validNickname(name[1]) {
		tc.send("You are now known as " + name[1])
		tc.nick = name[1]
		for _, k := range tc.TrebuchetBot.knownContacts {
			if k.Hostname == tc.Conn.OtherHostname {
				k.Nickname = tc.nick
			}
		}
	}
}

//validNickname checks for valid nicknames.
//TODO: unsteal from ricochet-go/core/sanitize.go once in go-ricochet or similar.
func validNickname(nickname string) bool {
	length := 0
	blacklist := []rune{'"', '<', '>', '&', '\\'}

	for len(nickname) > 0 {
		r, sz := utf8.DecodeRuneInString(nickname)
		if r == utf8.RuneError {
			return false
		}

		if unicode.In(r, unicode.Cf, unicode.Cc) {
			return false
		}

		for _, br := range blacklist {
			if r == br {
				return false
			}
		}

		length++
		if length > 20 {
			return false
		}
		nickname = nickname[sz:]
	}

	return length > 0
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
	log.Printf("Disconnect by %s (%s) \n", tc.Conn.OtherHostname, tc.nick)
	if tc.TrebuchetBot.JoinPartNotifications {
		for _, c := range tc.TrebuchetBot.activeContacts {
			c.send(tc.nick + " left the room.")
		}
	}
	tc.StandardRicochetConnection.OnDisconnect()
}
