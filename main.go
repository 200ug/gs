package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

const usage = `usage:
	gs init <user@host:port:/path>  initialize config with remote server
	gs track                        add current directory to sync list
	gs untrack                      remove current directory from sync list
	gs push                         sync local to server
	gs pull                         sync server to local
	gs status                       show pending changes (dry-run)
	gs auto [options]               wait for server, then pull all

auto options:
	--interval <duration>           poll interval (default: 30s)
	--timeout <duration>            max wait time, 0 for infinite (default: 15m)
`

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Print(usage)
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit()
	case "track":
		err = cmdTrack()
	case "untrack":
		err = cmdUntrack()
	case "push":
		err = cmdPush()
	case "pull":
		err = cmdPull()
	case "status":
		err = cmdStatus()
	case "auto":
		err = runAuto()
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Printf("[!] unknown command: %s\n", os.Args[1])
		fmt.Print(usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("[!] error during command execution: %s\n", err)
		os.Exit(1)
	}
}

func runInit() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: gs init <user@host:port:/path>")
	}
	return cmdInit(os.Args[2])
}

func runAuto() error {
	fs := flag.NewFlagSet("auto", flag.ExitOnError)
	interval := fs.Duration("interval", 30*time.Second, "poll interval")
	timeout := fs.Duration("timeout", 15*time.Minute, "max wait time (0 for infinite)")
	fs.Parse(os.Args[2:])
	return cmdAuto(*interval, *timeout)
}
