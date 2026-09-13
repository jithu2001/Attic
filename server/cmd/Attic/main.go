// Command Attic is the single Attic server binary: API, scanner, background
// jobs and media streaming in one modular monolith.
//
// Usage:
//
//	attic                      run the server
//	attic serve                run the server
//	attic adduser <username>   create an account
//	attic -healthcheck         probe a running server (used by the container)
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	args := os.Args[1:]

	// -healthcheck lets the container image probe itself without shipping
	// curl in the runtime layer (see Dockerfile HEALTHCHECK).
	if len(args) == 1 && (args[0] == "-healthcheck" || args[0] == "--healthcheck") {
		if err := probeHealth(); err != nil {
			fail("unhealthy: " + err.Error())
		}
		return
	}

	var (
		command string
		rest    []string
	)
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, rest = args[0], args[1:]
	} else {
		command, rest = "serve", args
	}

	var err error
	switch command {
	case "serve":
		err = runServer(rest)
	case "adduser":
		err = runAddUser(rest)
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		fail("unknown command " + command)
	}

	if err != nil {
		fail(err.Error())
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Attic — self-hosted photos, music and video.

Usage:
  attic [serve]              run the server
  attic adduser <username>   create an account (prompts for a password)
  attic -healthcheck         probe a running server

Configuration comes from the environment; see .env.example.
`)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "attic: "+message)
	os.Exit(1)
}

// probeHealth performs a local GET /healthz, used by the container healthcheck.
func probeHealth() error {
	addr := os.Getenv("ATTIC_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /healthz returned %s", resp.Status)
	}
	return nil
}
