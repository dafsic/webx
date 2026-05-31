package dao

import (
	"log/slog"
)

type DAO struct {
	logger *slog.Logger
}

func NewDAO(logger *slog.Logger) *DAO {
	return &DAO{
		logger: logger,
	}
}