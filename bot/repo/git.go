package repo

import (
	"errors"
	"log/slog"
	"os"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

func repoInit(path string, remote string, branch string) error {
	repo, err := git.PlainClone(path, &git.CloneOptions{
		URL:           remote,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
	})
	if repo != nil {
		_ = repo.Close()
	}

	return err
}

func CheckLatest(repoPath string, remote string, branch string) error {
	if _, err := os.Stat(repoPath); err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(repoPath, 0755); err != nil {
				return err
			}

			slog.Info("init local repo")

			return repoInit(repoPath, remote, branch)
		}

		return err
	}

	slog.Info("update local repo")

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}
	defer repo.Close()

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = wt.Pull(&git.PullOptions{
		RemoteName: "origin",
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	return nil
}
