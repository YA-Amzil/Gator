// Package state persists the currently logged-in user between CLI
// invocations. This is session data, not configuration, so it lives outside
// the environment-variable-based config in a small JSON file under the
// user's home directory.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	stateDirName  = ".gator"
	stateFileName = "session.json"
)

type Session struct {
	CurrentUserName string `json:"current_user_name"`
}

func path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, stateDirName, stateFileName), nil
}

// Read returns the saved session, or a zero-value Session if none exists yet.
func Read() (Session, error) {
	p, err := path()
	if err != nil {
		return Session{}, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Session{}, nil
		}
		return Session{}, fmt.Errorf("reading session file: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{}, fmt.Errorf("parsing session file: %w", err)
	}
	return s, nil
}

// Write persists the given session, creating the state directory if needed.
func Write(s Session) error {
	p, err := path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding session file: %w", err)
	}

	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}
	return nil
}
