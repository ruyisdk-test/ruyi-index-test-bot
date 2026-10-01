package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/pelletier/go-toml/v2"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/db"
)

const upstreamRepoId = "upstream"

type Riko2 struct {
	Upstream struct {
		Source     string `toml:"source" json:"source"`
		Github     string `toml:"github" json:"github"`
		WithMirror bool   `toml:"with_mirror" json:"with_mirror"`
	} `toml:"upstream" json:"upstream"`

	Mirror struct {
		Url []string `toml:"url" json:"url"`
	} `toml:"mirror" json:"mirror"`

	Packages map[string][]string `toml:"packages" json:"packages"`
}

type Upstream struct {
	Name   string `json:"name"`
	Riko2  Riko2  `json:"riko2"`
	Readme string `json:"-"`
}

var upstreamsConfig map[string]Upstream = nil
var packagesUpstream map[string]string = nil

const packagesKeyFmt string = "%s\x00%s"

func UpstreamLoad(repoPath string, dbTtlDays int64) error {
	upPath := filepath.Join(repoPath, "upstream")
	if _, err := os.Stat(upPath); err != nil {
		slog.Error("upstream configs not found", "path", upPath)
		return err
	}

	ups, err := os.ReadDir(upPath)
	if err != nil {
		slog.Error("upstream config dir read failed", "path", upPath)
		return err
	} else if len(ups) == 0 {
		slog.Error("upstream config dir empty", "path", upPath)
		return errors.New("upstream config dir empty")
	}

	sups := make(map[string]Upstream)
	pkgu := make(map[string]string)
	for _, up := range ups {
		upRiko := path.Join(upPath, up.Name(), "riko2.toml")
		upReadme := path.Join(upPath, up.Name(), "README.md")

		rikoRaw, err := os.ReadFile(upRiko)
		if err != nil {
			slog.Warn("riko config file read failed", "path", upRiko, "err", err)
			slog.Info("will skip:", "upstream", up.Name())
			continue
		}
		readmeRaw, err := os.ReadFile(upReadme)
		if err != nil {
			slog.Warn("readme prompt read failed", "path", upReadme, "err", err)
			slog.Info("will skip:", "upstream", up.Name())
			continue
		}

		// upstream config
		riko2 := Riko2{}
		err = toml.Unmarshal(rikoRaw, &riko2)
		if err != nil {
			slog.Warn("unmarshal riko config failed", "path", upRiko, "err", err)
			slog.Info("will skip:", "upstream", up.Name())
			continue
		}

		if !slices.Contains([]string{"web", "github"}, riko2.Upstream.Source) {
			slog.Warn("unsupported riko2 source:", "source", riko2.Upstream.Source)
			slog.Info("will skip:", "upstream", up.Name())
			continue
		} else if riko2.Upstream.Source == "github" {
			if riko2.Upstream.Github == "" {
				slog.Warn("riko2 source github with empty repo")
				slog.Info("will skip:", "upstream", up.Name())
				continue
			}
		} else if riko2.Upstream.Source == "web" {
			riko2.Upstream.WithMirror = true
		}

		if riko2.Upstream.WithMirror == true {
			if riko2.Mirror.Url == nil || len(riko2.Mirror.Url) == 0 {
				slog.Warn("riko2 source with mirror but met empty mirror list")
				slog.Info("will skip:", "upstream", up.Name())
				continue
			}
		}

		if riko2.Packages == nil || len(riko2.Packages) == 0 {
			slog.Warn("riko2 upstream with empty package list")
			slog.Info("will skip:", "upstream", up.Name())
			continue
		}

		sup := Upstream{
			Name:   up.Name(),
			Riko2:  riko2,
			Readme: string(readmeRaw),
		}
		_, err = MirrorApply(sup.Riko2.Mirror.Url)
		if err != nil {
			slog.Warn("riko2 mirror configs apply failed", "path", upRiko, "err", err)
			slog.Info("will skip:", "upstream", up.Name())
			continue
		}
		// check only
		// really apply on check
		// sup.Riko2.Mirror.Url = nu

		sups[up.Name()] = sup

		// package to upstream
		for t, ps := range sup.Riko2.Packages {
			for _, p := range ps {
				k := fmt.Sprintf(packagesKeyFmt, t, p)
				if v, ok := pkgu[k]; ok {
					return errors.New("duplicate upstream for package [" + k + "], upstream " + v + " and " + up.Name())
				}

				pkgu[k] = up.Name()
			}
		}
	}

	supsj := make(map[string]string)
	for v, k := range sups {
		j, err := json.Marshal(k)
		if err != nil {
			return err
		}
		supsj[v] = string(j)
	}

	err = db.AddUpstreamView(context.Background(), getRepoHash(upstreamRepoId), dbTtlDays, supsj, pkgu)
	if err != nil {
		return err
	}

	upstreamsConfig = sups
	packagesUpstream = pkgu

	return nil
}

func getUpstreamsMap() map[string]Upstream {
	return upstreamsConfig
}

func getUpstream(upstreamName string) (Upstream, error) {
	upstream, ok := upstreamsConfig[upstreamName]
	if !ok {
		return Upstream{}, errors.New("upstream not found")
	}
	return upstream, nil
}

func GetUpstreamByPackage(packageName string, packageGroup string) (Upstream, error) {
	ctx := context.Background()
	un, err := db.GetPackageUpstream(ctx, packageGroup, packageName)
	if err != nil {
		return Upstream{}, err
	}
	uj, err := db.GetUpstream(ctx, un)
	if err != nil {
		return Upstream{}, err
	}
	u := Upstream{}
	err = json.Unmarshal([]byte(uj), &u)
	if err != nil {
		return Upstream{}, err
	}
	return u, nil
}
