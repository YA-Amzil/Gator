package cli

import "fmt"

// Command is a parsed CLI invocation: the command name and its positional
// arguments.
type Command struct {
	Name string
	Args []string
}

type Handler func(s *State, cmd Command) error

// Commands is a registry mapping command names to their handlers.
type Commands struct {
	handlers map[string]Handler
}

func NewCommands() *Commands {
	return &Commands{handlers: make(map[string]Handler)}
}

func (c *Commands) Register(name string, f Handler) {
	c.handlers[name] = f
}

func (c *Commands) Run(s *State, cmd Command) error {
	handler, ok := c.handlers[cmd.Name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
	return handler(s, cmd)
}
