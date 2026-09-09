package repo

import "log/slog"

func TestRun(repoPath string) error {
	cfg, err := ConfigLoad(repoPath)
	if err != nil {
		return err
	}
	pkgs, err := PackagesLoad(repoPath)
	if err != nil {
		return err
	}

	slog.Info("load repo", "cfg", cfg, "pkgs", pkgs)

	return nil
}
