Ricochet Group Chat
===

A minimal group-chat bot for ricochet.

Usage:

> go run main.go bot.go -controlPort=127.0.0.1:9151

(or wherever your version of tor has its control port)

the onion key will be generated and printed to stdout.

The bot will auto-accept contact requests, and all of its contacts will receive
any chat messages sent to the bot.

Nicknames default to your onion address, but you can change the prefix by
sending the bot:

> /nick nickname
