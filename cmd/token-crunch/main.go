package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/micaelmalta/token-crunch/internal/config"
	"github.com/micaelmalta/token-crunch/internal/hook"
	"github.com/micaelmalta/token-crunch/internal/install"
	"github.com/micaelmalta/token-crunch/internal/session"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "pre":
		if err := hook.Pre(); err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch pre: %v\n", err)
			os.Exit(1)
		}
	case "post":
		if err := hook.Post(); err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch post: %v\n", err)
			os.Exit(1)
		}
	case "flush":
		if err := hook.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch flush: %v\n", err)
			os.Exit(1)
		}
	case "install":
		local := len(os.Args) > 2 && os.Args[2] == "--local"
		if err := install.Install(local); err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch install: %v\n", err)
			os.Exit(1)
		}
		if local {
			fmt.Println("token-crunch: hooks installed in .claude/settings.local.json")
		} else {
			fmt.Println("token-crunch: hooks installed in ~/.claude/settings.json")
		}
	case "uninstall":
		local := len(os.Args) > 2 && os.Args[2] == "--local"
		if err := install.Uninstall(local); err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch uninstall: %v\n", err)
			os.Exit(1)
		}
		if local {
			fmt.Println("token-crunch: hooks removed from .claude/settings.local.json")
		} else {
			fmt.Println("token-crunch: hooks removed from ~/.claude/settings.json")
		}
	case "stats":
		var err error
		if len(os.Args) > 2 && os.Args[2] == "--json" {
			err = session.StatsJSON()
		} else {
			err = session.Stats()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch stats: %v\n", err)
			os.Exit(1)
		}
	case "clean":
		days := 30
		if len(os.Args) > 2 {
			n, err := strconv.Atoi(os.Args[2])
			if err != nil {
				fmt.Fprintln(os.Stderr, "usage: token-crunch clean [days]")
				os.Exit(1)
			}
			days = n
		}
		if err := session.Clean(days); err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch clean: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("token-crunch: cleaned sessions older than %d day(s)\n", days)
	case "explain":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: token-crunch explain <payload.json|->")
			os.Exit(1)
		}
		if err := hook.Explain(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch explain: %v\n", err)
			os.Exit(1)
		}
	case "replay":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: token-crunch replay <log-file> [--json]")
			os.Exit(1)
		}
		var err error
		if len(os.Args) > 3 && os.Args[3] == "--json" {
			err = hook.ReplayJSON(os.Args[2])
		} else {
			err = hook.Replay(os.Args[2])
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "token-crunch replay: %v\n", err)
			os.Exit(1)
		}
	case "config":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: token-crunch config <validate|show>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "validate":
			path := os.Getenv("TOKEN_CRUNCH_CONFIG")
			if len(os.Args) > 3 {
				path = os.Args[3]
			}
			if path == "" {
				fmt.Fprintln(os.Stderr, "token-crunch config validate: no config file specified (set TOKEN_CRUNCH_CONFIG or pass path)")
				os.Exit(1)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "token-crunch config validate: %v\n", err)
				os.Exit(1)
			}
			var fc map[string]any
			if err := json.Unmarshal(data, &fc); err != nil {
				fmt.Fprintf(os.Stderr, "token-crunch config validate: invalid JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("token-crunch: %s is valid JSON\n", path)
		case "show":
			cfg := config.Load()
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
		default:
			fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", os.Args[2])
			os.Exit(1)
		}
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `token-crunch — context-aware token compression for Claude Code

Commands:
  install [--local]   write hooks into ~/.claude/settings.json (or .claude/settings.local.json)
  uninstall [--local] remove hooks from settings file
  pre                 PreToolUse hook entrypoint (reads stdin, writes stdout)
  post                PostToolUse hook entrypoint (reads stdin, writes stdout)
  flush               Stop hook entrypoint (persists session store)
  stats [--json]      show cumulative savings across sessions
  clean [days]        remove session files older than days (0 removes all)
  explain <file>      explain compression decision for a hook/replay payload
  replay <log> [--json]
                      replay session log with different thresholds
  config validate [file]  validate a JSON config file
  config show             print the active resolved config
  version             print token-crunch version`)
}
