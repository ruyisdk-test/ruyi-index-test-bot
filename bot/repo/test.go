package repo

import "log/slog"

func TestRun(repoPath string) error {
	err := LoadData(repoPath)

	if err != nil {
		return err
	}

	slog.Info("load repo")

	return nil
}
