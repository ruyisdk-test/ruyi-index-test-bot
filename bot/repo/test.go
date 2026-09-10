package repo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/db"
)

func testUrl(url string) (int, error) {
	// auto handle 302
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("http status: %s", resp.Status)
	}
	buf := make([]byte, 32*1024)

	_, err = io.ReadFull(resp.Body, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && err != io.EOF {
		return resp.StatusCode, fmt.Errorf("download failed: %w", err)
	}

	return resp.StatusCode, nil
}

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

	testList := db.GetUrlTestList()
	for _, url := range testList {
		code, err := testUrl(url)
		err = db.AddUrlTestStatus(ctx, ttlDays, url, code, err)
		if err != nil {
			return err
		}
	}

	return nil
}
