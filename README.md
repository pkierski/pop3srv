# pop3srv

[![Go Reference](https://pkg.go.dev/badge/github.com/pkierski/pop3srv.svg)](https://pkg.go.dev/github.com/pkierski/pop3srv)
[![rcard](https://goreportcard.com/badge/github.com/pkierski/pop3srv)](https://goreportcard.com/report/github.com/pkierski/pop3srv)

This package provides an implementation of a POP3 (Post Office Protocol v3) server in Go.

Despite of the POP3 isn't widely used nowadays, I've decided to write this package as a excersise.

## Basic usage

This package can be imported as usual in Go:
```bash
go get github.com/pkierski/pop3srv
```

Very basic server can be set up as following:
```go
package main

import "github.com/pkierski/pop3srv"

func main() {
	pop3srv.NewServer(pop3srv.AllowAllAuthorizer{}, pop3srv.EmptyMailboxProvider{}).
		ListenAndServe("")
}
```
This server starts listening on default POP3 port (110) and allows to open mailbox with any credentials,
but all mailboxes are empty.

For more information see [documentation](https://pkg.go.dev/github.com/pkierski/pop3srv).

## TODO
 * [x] check if message is deleted and return error for DELE, RETR, LIST and other commands
 * [x] limit concurrent sessions count
 * [x] add timeout for idle/stale sessions
 * [x] support for multiple `Server`/`ListenAndServe` calls
 * [x] replace switch/case in `Session.handle...State` with map of command.name -> handler method
 * [x] separate authorization interface for APOP and USER/PASS methods, return proper capabilities
 * [ ] unit tests
 * [ ] TLS implementation
 * [ ] consider use of log/slog
