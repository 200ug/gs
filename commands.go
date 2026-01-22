package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func cmdInit(remote string) error {
	if _, err := loadConfig(); err == nil {
		return fmt.Errorf("config already exists at %s", configPath())
	}

	host, port, remotePath, err := parseRemote(remote)
	if err != nil {
		return err
	}

	fmt.Printf("[~] checking server reachability... ")
	if !isServerReachable(host, port, 5*time.Second) {
		fmt.Println("failed")
		return fmt.Errorf("cannot reach server %s on port %s", host, port)
	}
	fmt.Println("ok")

	cfg := &Config{
		Server:     host,
		Port:       port,
		RemotePath: remotePath,
		Excludes:   []string{".git", "*.tmp", stateFileName},
	}

	if err := saveConfig(cfg); err != nil {
		return err
	}

	fmt.Printf("[+] initialized gs with remote %s:%s\n", cfg.Server, cfg.RemotePath)
	fmt.Printf("[+] config saved to %s\n", configPath())
	fmt.Println("[+] run 'gs track' in directories you want to sync")

	return nil
}

func cmdTrack() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("no config found (run 'gs init' first)")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	localName := filepath.Base(cwd)

	if err := cfg.AddLocal(localName, cwd); err != nil {
		return err
	}

	if err := saveConfig(cfg); err != nil {
		return err
	}

	fmt.Printf("[+] tracking '%s' (%s) -> %s\n", localName, cwd, cfg.RemoteForLocal(cfg.FindLocalByName(localName)))
	return nil
}

func cmdUntrack() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("no config found")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	local := cfg.FindLocalForPath(cwd)
	if local == nil {
		return fmt.Errorf("current directory is not tracked")
	}

	name := local.Name
	cfg.RemoveLocal(name)

	if err := saveConfig(cfg); err != nil {
		return err
	}

	fmt.Printf("[+] untracked '%s'\n", name)
	return nil
}

func getCurrentLocal(cfg *Config) (*Local, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	local := cfg.FindLocalForPath(cwd)
	if local == nil {
		return nil, fmt.Errorf("current directory is not a configured local (run 'gs track' first)")
	}
	return local, nil
}

func cmdPush() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	local, err := getCurrentLocal(cfg)
	if err != nil {
		return err
	}
	remote := cfg.RemoteForLocal(local)

	state, err := loadState(local.Path)
	if err != nil {
		return err
	}

	if !state.LastPull.IsZero() {
		fmt.Println("[~] checking for remote changes...")
		changes, err := checkRemoteChanges(cfg, local)
		if err != nil {
			return fmt.Errorf("failed to check remote: %w", err)
		}
		if len(changes) > 0 {
			fmt.Println("[!] warning: remote has changes that haven't been pulled:")
			for _, c := range changes {
				fmt.Printf("  %s\n", c)
			}
			fmt.Println("[+] consider running 'gs pull' first, or use 'gs push' again to force")
		}
	}

	if state.LastPush.IsZero() {
		fmt.Println("[!] warning: first push will use --delete flag, which removes files on remote that don't exist locally")
		fmt.Println("[?] press ctrl+c to abort, or wait 5 seconds to continue...")
		time.Sleep(5 * time.Second)
	}

	fmt.Printf("[~] pushing '%s' to server...\n", local.Name)
	result, err := runRsync(local.Path, remote, cfg.Port, cfg.Excludes, false, true)
	if err != nil {
		return err
	}

	state.LastPush = time.Now().UTC()
	if err := saveState(local.Path, state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Print(result.Output)
	fmt.Println("[+] push complete")
	return nil
}

func cmdPull() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	local, err := getCurrentLocal(cfg)
	if err != nil {
		return err
	}
	remote := cfg.RemoteForLocal(local)

	state, err := loadState(local.Path)
	if err != nil {
		return err
	}

	if state.LastPull.IsZero() {
		fmt.Println("[!] warning: first pull will use --delete flag, which removes local files that don't exist on remote")
		fmt.Println("[?] press ctrl+c to abort, or wait 3 seconds to continue...")
		time.Sleep(3 * time.Second)
	}

	fmt.Printf("[~] pulling '%s' from server...\n", local.Name)
	result, err := runRsync(remote, local.Path, cfg.Port, cfg.Excludes, false, true)
	if err != nil {
		return err
	}

	state.LastPull = time.Now().UTC()
	if err := saveState(local.Path, state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Print(result.Output)
	fmt.Println("[+] pull complete")
	return nil
}

func cmdStatus() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	local, err := getCurrentLocal(cfg)
	if err != nil {
		return err
	}
	remote := cfg.RemoteForLocal(local)

	fmt.Printf("[~] checking status for '%s'...\n", local.Name)

	fmt.Println("[~] checking remote...")
	pullResult, err := runRsync(remote, local.Path, cfg.Port, cfg.Excludes, true, true)
	if errors.Is(err, ErrRemoteNotFound) {
		fmt.Println("[!] remote directory does not exist yet")
		fmt.Println("[+] run 'gs push' to initialize it")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check remote: %w", err)
	}

	fmt.Println("[~] checking local...")
	pushResult, err := runRsync(local.Path, remote, cfg.Port, cfg.Excludes, true, true)
	if err != nil {
		return fmt.Errorf("failed to check local changes: %w", err)
	}

	fmt.Println()
	if len(pushResult.Changes) == 0 && len(pullResult.Changes) == 0 {
		fmt.Println("[+] everything is in sync")
		return nil
	}

	if len(pushResult.Changes) > 0 {
		fmt.Println("local changes (push to sync):")
		for _, c := range pushResult.Changes {
			fmt.Printf("  %s\n", c)
		}
	}

	if len(pullResult.Changes) > 0 {
		fmt.Println("remote changes (pull to sync):")
		for _, c := range pullResult.Changes {
			fmt.Printf("  %s\n", c)
		}
	}

	return nil
}

func pullLocal(cfg *Config, local *Local) error {
	remote := cfg.RemoteForLocal(local)

	state, err := loadState(local.Path)
	if err != nil {
		return err
	}

	fmt.Printf("[~] pulling '%s' from server...\n", local.Name)
	result, err := runRsync(remote, local.Path, cfg.Port, cfg.Excludes, false, true)
	if err != nil {
		return err
	}

	state.LastPull = time.Now().UTC()
	if err := saveState(local.Path, state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	fmt.Print(result.Output)
	fmt.Printf("[+] pull complete for '%s'\n", local.Name)
	return nil
}

func cmdAuto(interval, timeout time.Duration) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if len(cfg.Locals) == 0 {
		return fmt.Errorf("no locals configured")
	}

	fmt.Printf("[~] waiting for server %s:%s...\n", cfg.Server, cfg.Port)
	if err := waitForServer(cfg.Server, cfg.Port, interval, timeout); err != nil {
		return err
	}

	fmt.Printf("[~] server is reachable, pulling %d local(s)...\n", len(cfg.Locals))

	var failed []string
	for i := range cfg.Locals {
		if err := pullLocal(cfg, &cfg.Locals[i]); err != nil {
			fmt.Printf("[!] failed to pull '%s': %s\n", cfg.Locals[i].Name, err)
			failed = append(failed, cfg.Locals[i].Name)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to pull: %v", failed)
	}

	fmt.Println("[+] auto-pull complete for all locals")
	return nil
}
