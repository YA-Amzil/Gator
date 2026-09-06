package cli

import (
	"gator/internal/cache"
	"gator/internal/config"
	"gator/internal/database"
	"gator/internal/state"
)

// State holds the dependencies every command handler needs.
type State struct {
	DB    *database.Queries
	Cfg   *config.Config
	Users *cache.UserCache
}

func (s *State) Session() (state.Session, error) {
	return state.Read()
}
