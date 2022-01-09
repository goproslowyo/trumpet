// Parses the commands sent through Discord. This program does not use any
// traditional command line options, as it uses config.json for configuration.
package main

import (
	"strings"
)

// CmdGetArgs returns a false and a nil slice if cmd was not intended for the bot.
func CmdGetArgs(cmd string) (args []string, ok bool) {
	if !strings.HasPrefix(cmd, cfg.Prefix) {
		return nil, false
	}
	cmd = strings.TrimPrefix(cmd, cfg.Prefix)
	return strings.Fields(cmd), true
}
