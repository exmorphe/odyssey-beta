package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	configDir := DefaultDir()

	switch os.Args[1] {
	case "help", "--help", "-h":
		printUsage()
		return

	case "login":
		serverURL := ""
		if len(os.Args) >= 3 {
			serverURL = os.Args[2]
		} else {
			cfg, err := LoadConfig(configDir)
			if err == nil && cfg.Server != "" {
				serverURL = cfg.Server
			}
		}
		if serverURL == "" {
			fmt.Fprintln(os.Stderr, "Usage: ody login <server-url>")
			os.Exit(1)
		}
		if err := runLogin(serverURL, configDir, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "start":
		client, err := loadClient(configDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := runStart(client, &KubectlRunner{}, &RealKindManager{}, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "verify":
		client, err := loadClient(configDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := runVerify(client, &KubectlRunner{}, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "status":
		client, err := loadClient(configDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := runStatus(client, &RealKindManager{}, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "clue":
		client, err := loadClient(configDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := runClue(client, &KubectlRunner{}, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "feedback":
		client, err := loadClient(configDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		message, exID := parseFeedbackArgs(os.Args[2:])
		if message == "" {
			fmt.Fprintln(os.Stderr, "Usage: ody feedback \"<message>\" [--exercise <id>]")
			os.Exit(1)
		}
		if err := runFeedback(client, message, exID, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func loadClient(configDir string) (*Client, error) {
	cfg, err := LoadConfig(configDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if cfg.Server == "" {
		return nil, fmt.Errorf("not logged in — run `ody login <server-url>`")
	}
	return NewClient(cfg, configDir), nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: ody <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  login    <server-url>  Authenticate via OAuth device flow")
	fmt.Fprintln(os.Stderr, "  start                  Fetch and apply the active exercise")
	fmt.Fprintln(os.Stderr, "  verify                 Capture cluster state and check faults")
	fmt.Fprintln(os.Stderr, "  status                 Show current exercise state")
	fmt.Fprintln(os.Stderr, "  clue                   Show diagnostic clue for the active exercise")
	fmt.Fprintln(os.Stderr, "  feedback \"<message>\"    Leave feedback on the current exercise")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Report a bug:    https://github.com/exmorphe/odyssey-beta/issues/new/choose")
	fmt.Fprintln(os.Stderr, "Discussions:     https://github.com/exmorphe/odyssey-beta/discussions")
	fmt.Fprintln(os.Stderr, "Quick feedback:  ody feedback \"...\"")
}
