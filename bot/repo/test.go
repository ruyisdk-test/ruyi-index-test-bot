package repo

import "log/slog"

func TestRun(repoPath string, ttlDays int64) error {
	err := LoadData(repoPath, ttlDays)

	if err != nil {
		return err
	}

	slog.Info("load repo")

	return nil
}
