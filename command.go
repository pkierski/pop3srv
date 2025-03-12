package pop3srv

import (
	"strconv"
	"strings"
)

type command struct {
	name    string
	args    []string
	numArgs []int
}

const (
	userCmd = "USER"
	passCmd = "PASS"
	statCmd = "STAT"
	listCmd = "LIST"
	retrCmd = "RETR"
	deleCmd = "DELE"
	noopCmd = "NOOP"
	rsetCmd = "RSET"
	quitCmd = "QUIT"
	apopCmd = "APOP"
	topCmd  = "TOP"
	uidlCmd = "UIDL"
	capaCmd = "CAPA"
)

func (c *command) oneNumArg() bool {
	return len(c.args) == 1 && c.numArgs[0] != -1
}

func (c *command) twoNumArgs() bool {
	return len(c.args) == 2 && c.numArgs[0] != -1 && c.numArgs[1] != -1
}

func (c *command) parse(line string) {
	parts := strings.SplitN(line, " ", 3)
	c.name = strings.ToUpper(parts[0])
	c.args = parts[1:]
	c.numArgs = make([]int, len(c.args))
	for i, arg := range c.args {
		numArg, err := strconv.Atoi(arg)
		if err == nil {
			c.numArgs[i] = numArg
			if i == 0 && c.numArgs[i] > 0 {
				c.numArgs[i] -= 1
			}
		} else {
			c.numArgs[i] = -1
		}
	}
}
