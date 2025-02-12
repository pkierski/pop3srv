# pop3srv
POP3 server module in Go

## TODO
 * [x] check if message is deleted and return error for DELE, RETR, LIST and other commands
 * [x] limit sessions count
 * [x] add timeout for idle/stale sessions
 * [x] support for multiple `Server`/`ListenAndServe` calls