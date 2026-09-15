package repo

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

var repoHash = make(map[string]string)

func setRepoHash(id string, hash string) {
	if len(hash) > 7 {
		repoHash[id] = hash[:7]
	}
}

func getRepoHash(id string) string {
	if repoHash[id] == "" {
		slog.Warn("get empty local repo hash:", "id", id)
		return fmt.Sprintf("test-dirty-%s", id)
	}
	return repoHash[id]
}

func repoInit(id string, path string, remote string, branch string) error {
	repo, err := git.PlainClone(path, &git.CloneOptions{
		URL:           remote,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
	})

	if err != nil {
		return err
	}
	defer repo.Close()

	hash, err := repo.Head()
	if err != nil {
		return err
	}
	setRepoHash(id, hash.Hash().String())

	return nil
}

func CheckLatest(id string, repoPath string, remote string, branch string) error {
	if _, err := os.Stat(repoPath); err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(repoPath, 0755); err != nil {
				return err
			}

			slog.Info("init local repo:", "id", id)

			return repoInit(id, repoPath, remote, branch)
		}

		return err
	}

	slog.Info("update local repo:", "id", id)

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

	hash, err := repo.Head()
	if err != nil {
		return err
	}
	setRepoHash(id, hash.Hash().String())

	return nil
}
