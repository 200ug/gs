package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type State struct {
	LastPush time.Time
	LastPull time.Time
}

const stateFileName = ".gs.state"

func statePath(localDir string) string {
	return filepath.Join(localDir, stateFileName)
}

func loadState(localDir string) (*State, error) {
	path := statePath(localDir)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open state file: %w", err)
	}
	defer f.Close()

	state := &State{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		t, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			continue
		}

		switch parts[0] {
		case "last_push":
			state.LastPush = t
		case "last_pull":
			state.LastPull = t
		}
	}

	return state, scanner.Err()
}

func saveState(localDir string, state *State) error {
	path := statePath(localDir)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create state file: %w", err)
	}
	defer f.Close()

	if !state.LastPush.IsZero() {
		fmt.Fprintf(f, "last_push %s\n", state.LastPush.Format(time.RFC3339))
	}
	if !state.LastPull.IsZero() {
		fmt.Fprintf(f, "last_pull %s\n", state.LastPull.Format(time.RFC3339))
	}

	return nil
}
