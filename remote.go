package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

var ErrRemoteNotFound = errors.New("remote directory does not exist")

const sshOptions = "-o PasswordAuthentication=no -o BatchMode=yes"

func isServerReachable(host, port string, timeout time.Duration) bool {
	hostOnly := host
	if at := strings.LastIndex(host, "@"); at != -1 {
		hostOnly = host[at+1:]
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(hostOnly, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

func waitForServer(host, port string, interval, timeout time.Duration) error {
	start := time.Now()
	for {
		if isServerReachable(host, port, 5*time.Second) {
			return nil
		}

		if timeout > 0 && time.Since(start) >= timeout {
			return fmt.Errorf("timeout waiting for server")
		}

		time.Sleep(interval)
	}
}

type RsyncResult struct {
	Output  string
	Changes []string
}

func runRsync(src, dst, port string, excludes []string, dryRun, del bool) (*RsyncResult, error) {
	sshCmd := fmt.Sprintf("ssh -p %s %s", port, sshOptions)
	args := []string{"-avz", "-e", sshCmd}

	if dryRun {
		args = append(args, "--dry-run", "--itemize-changes")
	}

	if del {
		args = append(args, "--delete")
	}

	if len(excludes) > 0 {
		excludeFile, err := writeExcludeFile(excludes)
		if err != nil {
			return nil, err
		}
		defer os.Remove(excludeFile)
		args = append(args, "--exclude-from="+excludeFile)
	}

	if !strings.HasSuffix(src, "/") {
		src += "/"
	}
	if !strings.HasSuffix(dst, "/") {
		dst += "/"
	}

	args = append(args, src, dst)

	cmd := exec.Command("rsync", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			switch exitErr.ExitCode() {
			case 255: // ssh timeout
				return nil, fmt.Errorf("ssh connection failed (check pubkey auth): %s", string(output))
			case 23: // partial transfer error
				if strings.Contains(string(output), "No such file or directory") {
					return nil, ErrRemoteNotFound
				}
			}
		}
		return nil, fmt.Errorf("rsync failed: %w\n%s", err, string(output))
	}

	result := &RsyncResult{Output: string(output)}
	if dryRun {
		result.Changes = parseItemizedChanges(string(output))
	}

	return result, nil
}

func writeExcludeFile(excludes []string) (string, error) {
	f, err := os.CreateTemp("", "gs-excludes-*")
	if err != nil {
		return "", err
	}
	defer f.Close()

	for _, pattern := range excludes {
		fmt.Fprintln(f, pattern)
	}

	return f.Name(), nil
}

func parseItemizedChanges(output string) []string {
	var changes []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 12 {
			continue
		}
		if line[0] == '<' || line[0] == '>' || line[0] == '*' || line[0] == 'c' {
			changes = append(changes, line)
		}
	}
	return changes
}

func checkRemoteChanges(cfg *Config, local *Local) ([]string, error) {
	remote := cfg.RemoteForLocal(local)
	result, err := runRsync(remote, local.Path, cfg.Port, cfg.Excludes, true, false)
	if err != nil {
		return nil, err
	}
	return result.Changes, nil
}
