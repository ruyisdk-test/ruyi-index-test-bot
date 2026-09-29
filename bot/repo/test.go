package repo

import (
	"context"
	"log/slog"
	"slices"

	testagent "github.com/ruyisdk-test/ruyi-index-test-bot/bot/agent"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/db"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/static/prompt"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/web"
	"golang.org/x/mod/semver"
)

func DistfileUrlTestRun(repoPath string, ttlDays int64) error {
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

func UpstreamVersionTestRun() error {
	for n, c := range getUpstreamsMap() {
		prompt := []string{c.Readme}
		prompt = append(prompt, testprompt.UpstreamRawPrompt)

		slog.Info("version check prompt for ...:", "name", n)

		if c.Riko2.Upstream.WithMirror {
			for _, u := range c.Riko2.Mirror.Url {
				nu, err := MirrorApply([]string{u})
				if err != nil {
					slog.Error("failed to apply mirror url:", "url", u, "err", err)
					continue
				} else if len(nu) == 0 {
					slog.Error("url list empty after apply mirror url:", "url", u, "err", err)
					continue
				}

				prompt = append(prompt, nu[0])
				r, err := web.PageGet(nu[0])
				if err != nil {
					slog.Error("failed to get web page:", "url", nu[0], "err", err)
					continue
				}
				prompt = append(prompt, "```", string(r), "```")
			}
		}

		if c.Riko2.Upstream.Source == "web" {
			// do nothing special
		} else if c.Riko2.Upstream.Source == "github" {
			prompt = append(prompt, web.ReleasesGetRawPrompt(c.Riko2.Upstream.Github))
			r, err := web.ReleasesGetRaw(c.Riko2.Upstream.Github)
			if err != nil {
				slog.Error("failed to get github release:", "repo", c.Riko2.Upstream.Github, "err", err)
				continue
			}
			prompt = append(prompt, "```", r, "```")
		} else {
			slog.Error("unknown upstream source:", "name", n, "source", c.Riko2.Upstream.Source)
			continue
		}

		prompt = append(prompt, "下面给出该上游已经打包的至多十个最高版本：")
		if len(c.Riko2.Packages) == 0 {
			slog.Error("packages list empty")
			continue
		}
		oldv := make(map[string]string)
		semv := make([]string, 0)
		for g, pl := range c.Riko2.Packages {
			for _, p := range pl {
				vv, err := db.GetPackageVersions(context.Background(), p, g)
				if err != nil {
					slog.Error("failed to get old package versions:", "pkg", p, "group", g, "err", err)
					continue
				}
				for v, d := range vv["versions"].(map[string]db.VersionData) {
					if _, ok := oldv[v]; !ok {
						oldv[v] = d.UpstreamVersion

						// add leading v and sort with semver pkg later
						sv := "v" + v
						if !semver.IsValid(sv) {
							slog.Error("invalid semver version???", "value", sv)
							slog.Warn("maybe better ignore this error here")
						}
						semv = append(semv, sv)
					} else {
						if oldv[v] != d.UpstreamVersion {
							slog.Error("same upstream, version and upstreamversion miss match?", "version", v, "up ver1", oldv[v], "up ver2", d.UpstreamVersion)
							slog.Warn("will ignore this error")
						}
					}
				}
			}
		}
		semver.Sort(semv)
		slices.Reverse(semv)
		if len(semv) > 10 {
			semv = semv[:10]
		}
		for _, sv := range semv {
			prompt = append(prompt, oldv[sv[1:]])
		}
		prompt = append(prompt, "")

		// make prompt string
		prompts := prompt[0]
		for _, p := range prompt[1:] {
			prompts = prompts + "\n" + p
		}

		slog.Info("ask agent:", "prompt", prompts)
		// ask agent
		res, _, err := testagent.ModelAskUpstreamVersion(prompts)
		if err != nil {
			slog.Error("failed to ask agent:", "err", err)
		}
		slog.Info("agent answer:", "ans", res)
	}

	return nil
}
