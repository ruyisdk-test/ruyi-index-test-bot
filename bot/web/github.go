package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v92/github"
)

var ghClient *github.Client = nil

func InitGithubClient(pat string) error {
	client, err := github.NewClient(github.WithAuthToken(pat))
	if err != nil {
		return err
	}

	ghClient = client

	return nil
}

// ListFoxOrgs for pat valid check
func ListFoxOrgs() error {
	_, _, err := ghClient.Organizations.List(context.Background(), "weilinfox", nil)

	return err
}

func ReleasesGetRawPrompt(repo string) string {
	return fmt.Sprintf("从 API 获取 GitHub 仓库 %s 最新 25 个 release 的 json 结果", repo)
}

func ReleasesGetRaw(repo string) (string, error) {
	if ghClient == nil {
		return "", fmt.Errorf("github client not initialized")
	}

	p := strings.Split(repo, "/")
	if len(p) != 2 {
		return "", fmt.Errorf("invalid repo: %s", p)
	}

	u := fmt.Sprintf("repos/%s/%s/releases?per_page=25&page=1", p[0], p[1])

	ctx := context.WithValue(context.Background(), github.SleepUntilPrimaryRateLimitResetWhenRateLimited, true)

	req, err := ghClient.NewRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if _, err := ghClient.Do(req, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}
