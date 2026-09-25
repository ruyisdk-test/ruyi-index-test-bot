package repo

import (
	"context"
	"log/slog"

	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/db"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/web"
)

func TestRun(repoPath string, ttlDays int64) error {
	err := LoadIndexData(repoPath, ttlDays)

	if err != nil {
		return err
	}

	ctx := context.Background()

	testList := db.GetUrlTestList()
	for _, url := range testList {
		slog.Debug("run test for:", "url", url)
		code, err := web.TestUrl(url)
		err = db.AddUrlTestStatus(ctx, ttlDays, url, code, err)
		if err != nil {
			return err
		}
	}

	err = db.CleanUrlFailures(ctx)
	if err != nil {
		return err
	}

	return nil
}
