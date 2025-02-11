# pop3srv
POP3 server module in Go

## TODO
 * [ ] check if message is deleted and return error for DELE, RETR, LIST and other commands
 * [x] limit sessions count
 * [ ] add timeout for idle/stale sessions
 * [ ] support for multiple `Server`/`ListenAndServe` calls