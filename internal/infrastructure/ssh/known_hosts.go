package ssh

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"golang.org/x/crypto/ssh"
)

type HostKeyChecker struct {
	knownHostsPath string
	autoAddEnabled bool
}

func NewHostKeyChecker(knownHostsPath string) *HostKeyChecker {
	if knownHostsPath == "" {
		knownHostsPath = "/app/data/ssh/known_hosts"
	}

	sshDir := filepath.Dir(knownHostsPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		logger.Get().Warn().Err(err).Msg("Failed to create .ssh directory")
	}

	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		file, err := os.Create(knownHostsPath)
		if err != nil {
			logger.Get().Warn().Err(err).Msg("Failed to create known_hosts file")
		} else {
			file.Close()
			logger.Get().Info().Str("path", knownHostsPath).Msg("Created new known_hosts file")
		}
	}

	return &HostKeyChecker{
		autoAddEnabled: true,
		knownHostsPath: knownHostsPath,
	}
}

func (c *HostKeyChecker) EnableAutoAdd(enabled bool) {
	c.autoAddEnabled = enabled
}

func (c *HostKeyChecker) CheckHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	logger.Get().Debug().
		Str("known_hosts_path", c.knownHostsPath).
		Msg("Checking known_hosts file")

	c.dumpKnownHosts()

	found, matched := c.findHostKey(hostname, key)

	logger.Get().Debug().
		Str("hostname", hostname).
		Bool("found", found).
		Bool("matched", matched).
		Str("fingerprint", ssh.FingerprintSHA256(key)).
		Msg("Host key search result")

	if found && matched {
		logger.Get().Debug().
			Str("hostname", hostname).
			Msg("Host key found and matched - trusted")
		return nil
	}

	if found && !matched {
		logger.Get().Error().
			Str("hostname", hostname).
			Str("fingerprint", ssh.FingerprintSHA256(key)).
			Msg("HOST KEY MISMATCH! Connection rejected")
		return fmt.Errorf("host key mismatch for %s", hostname)
	}

	if !found && c.autoAddEnabled {
		logger.Get().Info().
			Str("hostname", hostname).
			Msg("First connection - adding host key to known_hosts")

		if err := c.addHostKey(hostname, key); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("hostname", hostname).
				Msg("Failed to add host key")
			return err
		}

		logger.Get().Info().
			Str("hostname", hostname).
			Msg("Host key added - future connections will be trusted")

		c.dumpKnownHosts()
		return nil
	}

	return fmt.Errorf("host key not found for %s", hostname)
}

func (c *HostKeyChecker) dumpKnownHosts() {
	file, err := os.Open(c.knownHostsPath)
	if err != nil {
		logger.Get().Debug().
			Err(err).
			Str("path", c.knownHostsPath).
			Msg("Failed to open known_hosts for dump")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	var lines []string

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line != "" {
			lines = append(lines, fmt.Sprintf("  %d: %s", lineNum, line))
		}
	}

	if len(lines) > 0 {
		logger.Get().Debug().
			Str("path", c.knownHostsPath).
			Int("entries", len(lines)).
			Strs("content", lines).
			Msg("Known_hosts file content")
	} else {
		logger.Get().Debug().
			Str("path", c.knownHostsPath).
			Msg("Known_hosts file is empty")
	}
}

func (c *HostKeyChecker) findHostKey(hostname string, key ssh.PublicKey) (bool, bool) {
	file, err := os.Open(c.knownHostsPath)
	if err != nil {
		logger.Get().Debug().
			Err(err).
			Str("path", c.knownHostsPath).
			Msg("Failed to open known_hosts for search")
		return false, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	keyStr := base64.StdEncoding.EncodeToString(key.Marshal())

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		hostPatterns := strings.Split(fields[0], ",")
		hostMatched := false
		for _, pattern := range hostPatterns {
			if c.matchHost(pattern, hostname) {
				hostMatched = true
				break
			}
		}
		if !hostMatched {
			continue
		}

		if len(fields) >= 3 && fields[2] == keyStr {
			logger.Get().Debug().
				Str("hostname", hostname).
				Str("line", line).
				Msg("Found matching host key")
			return true, true
		}

		logger.Get().Warn().
			Str("hostname", hostname).
			Str("expected_key", keyStr[:20]+"...").
			Str("found_key", fields[2][:20]+"...").
			Msg("Host matches but key differs")
		return true, false
	}

	return false, false
}

func (c *HostKeyChecker) addHostKey(hostname string, key ssh.PublicKey) error {
	sshDir := filepath.Dir(c.knownHostsPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	file, err := os.OpenFile(c.knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open known_hosts: %w", err)
	}
	defer file.Close()

	keyLine := fmt.Sprintf("%s %s %s\n", hostname, key.Type(), base64.StdEncoding.EncodeToString(key.Marshal()))

	if _, err := file.WriteString(keyLine); err != nil {
		return fmt.Errorf("failed to write to known_hosts: %w", err)
	}

	logger.Get().Debug().
		Str("hostname", hostname).
		Str("key_type", key.Type()).
		Msg("Appended host key to known_hosts")

	return nil
}

func (c *HostKeyChecker) matchHost(pattern, hostname string) bool {
	if pattern == hostname {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if strings.HasPrefix(hostname, parts[0]) && strings.HasSuffix(hostname, parts[len(parts)-1]) {
			return true
		}
	}
	return false
}
