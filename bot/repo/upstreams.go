package repo

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const upstreamRepoId = "upstream"

type Riko2 struct {
	Upstream struct {
		Source string `yaml:"source"`
		Github string `yaml:"github"`
	} `yaml:"upstream"`

	Mirror struct {
		Url []string `yaml:"url"`
	} `yaml:"mirror"`

	Packages map[string][]string `yaml:"packages"`
}

type Upstream struct {
	Riko2  Riko2
	Readme string
}

var upstreamsConfig map[string]Upstream = nil
var packagesUpstream map[string]string = nil

const packagesKeyFmt string = "%s\x00%s"

func UpstreamLoad(repoPath string) error {
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

		sup := Upstream{
			Riko2:  riko2,
			Readme: string(readmeRaw),
		}
		nu, err := MirrorApply(sup.Riko2.Mirror.Url)
		if err != nil {
			slog.Warn("riko2 mirror configs apply failed", "path", upRiko, "err", err)
			slog.Info("will skip:", "upstream", up.Name())
			continue
		}
		sup.Riko2.Mirror.Url = nu

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

	upstreamsConfig = sups
	packagesUpstream = pkgu

	return nil
}

func GetUpstream(upstreamName string) (Upstream, error) {
	upstream, ok := upstreamsConfig[upstreamName]
	if !ok {
		return Upstream{}, errors.New("upstream not found")
	}
	return upstream, nil
}

func GetUpstreamByPackage(packageName string, packageGroup string) (Upstream, error) {
	key := fmt.Sprintf(packagesKeyFmt, packageGroup, packageName)
	upstream, ok := packagesUpstream[key]
	if !ok {
		return Upstream{}, errors.New("package not found")
	}
	return GetUpstream(upstream)
}
