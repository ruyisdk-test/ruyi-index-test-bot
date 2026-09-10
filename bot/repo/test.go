package repo

import (
	"context"

	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/db"
)

func TestRun(repoPath string, ttlDays int64) error {
	err := LoadData(repoPath, ttlDays)

	if err != nil {
		return err
	}

	ctx := context.Background()
	err = db.CleanUrlFailures(ctx)
	if err != nil {
		return err
	}

	return nil
}
