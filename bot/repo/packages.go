package repo

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/db"
)

const indexRepoId = "ruyisdk"

type IndexConfig struct {
	RepoVersion string `toml:"ruyi-repo"`
	Repo        struct {
		Id   string `toml:"id"`
		Doc  string `toml:"doc_uri"`
		Name string `toml:"name"`
	} `toml:"repo"`

	Telemetry []Telemetry `toml:"telemetry"`

	Mirrors []Mirrors `toml:"mirrors"`
}

type Telemetry struct {
	Id    string `toml:"id"`
	Scope string `toml:"scope"`
	Url   string `toml:"url"`
}

type Mirrors struct {
	Id   string   `toml:"id"`
	Urls []string `toml:"urls"`
}

type PackageGroups struct {
	Name     string
	Packages []Packages
}

type Packages struct {
	Name     string
	Versions []Versions
}

type Versions struct {
	Name   string `toml:"-"`
	Format string `toml:"format"`

	Metadata VersionMetadata `toml:"metadata"`

	Distfiles []VersionDistfile `toml:"distfiles"`
}

type VersionMetadata struct {
	Description string `toml:"desc"`
	Vendor      struct {
		Name string `toml:"name"`
		Eula string `toml:"eula"`
	} `toml:"vendor"`
	UpstreamVersion string `toml:"upstream_version"`
}

type VersionDistfile struct {
	Name     string          `toml:"name"`
	Size     int64           `toml:"size"`
	Urls     []string        `toml:"urls"`
	Restrict []string        `toml:"restrict"`
	Checksum VersionChecksum `toml:"checksums"`
}

type VersionBinary struct {
	Host      string   `toml:"host"`
	Distfiles []string `toml:"distfiles"`

	Metadata struct {
		InstallSize int64 `toml:"install_size"`
	} `toml:"metadata"`
}

type VersionChecksum struct {
	Sha256 string `toml:"sha256"`
	Sha512 string `toml:"sha512"`
}

// IndexConfigLoad load index config from repoPath/config.toml
func IndexConfigLoad(repoPath string) (*IndexConfig, error) {
	configPath := filepath.Join(repoPath, "config.toml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := IndexConfig{}
	if err = toml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

var repoConfig *IndexConfig = nil
var repoPackages []PackageGroups = nil
var dbPackagesData map[string]map[string]map[string]db.VersionData = nil

func packagesIndexLoad(repoPath string) ([]PackageGroups, error) {
	config, err := IndexConfigLoad(repoPath)
	if err != nil {
		return nil, err
	}
	repoConfig = config

	repoMirrors := make(map[string][]string)
	for _, mirror := range repoConfig.Mirrors {
		repoMirrors[mirror.Id] = mirror.Urls
	}

	dbData := make(map[string]map[string]map[string]db.VersionData)

	packagesPath := filepath.Join(repoPath, "packages")
	if _, err := os.Stat(packagesPath); os.IsNotExist(err) {
		packagesPath = filepath.Join(repoPath, "manifests")
		if _, err := os.Stat(packagesPath); os.IsNotExist(err) {
			return nil, errors.New("no packages/manifests directory found in repo")
		}
	}

	entries, err := os.ReadDir(packagesPath)
	if err != nil {
		return nil, err
	}

	groups := make([]PackageGroups, len(entries))
	for i, entry := range entries {
		groups[i].Name = entry.Name()
		dbData[entry.Name()] = make(map[string]map[string]db.VersionData)

		pkgentries, err := os.ReadDir(filepath.Join(packagesPath, entry.Name()))
		if err != nil {
			return nil, err
		}
		groups[i].Packages = make([]Packages, len(pkgentries))

		for j, pkgentry := range pkgentries {
			groups[i].Packages[j].Name = pkgentry.Name()
			dbData[entry.Name()][pkgentry.Name()] = make(map[string]db.VersionData)

			verentries, err := os.ReadDir(filepath.Join(packagesPath, entry.Name(), pkgentry.Name()))
			if err != nil {
				return nil, err
			}
			groups[i].Packages[j].Versions = make([]Versions, len(verentries))

			for k, verentry := range verentries {
				vName := strings.TrimSuffix(verentry.Name(), filepath.Ext(verentry.Name()))
				groups[i].Packages[j].Versions[k].Name = vName

				data, err := os.ReadFile(filepath.Join(packagesPath, entry.Name(), pkgentry.Name(), verentry.Name()))
				if err != nil {
					return nil, err
				}

				if err = toml.Unmarshal(data, &groups[i].Packages[j].Versions[k]); err != nil {
					return nil, err
				}

				dbData[entry.Name()][pkgentry.Name()][vName] = db.VersionData{
					Distfiles:       make(map[string][]string),
					UpstreamVersion: groups[i].Packages[j].Versions[k].Metadata.UpstreamVersion,
				}
				for _, distfile := range groups[i].Packages[j].Versions[k].Distfiles {
					rm := slices.Contains(distfile.Restrict, "mirror")
					newUrls, err := ApplyIndexConfigUrl(distfile.Name, distfile.Urls, rm, repoMirrors)
					if err != nil {
						return nil, err
					}

					distfile.Urls = newUrls
					dbData[entry.Name()][pkgentry.Name()][vName].Distfiles[distfile.Name] = newUrls
				}
			}
		}
	}

	dbPackagesData = dbData

	return groups, nil
}

// ApplyIndexConfigUrl (distfile name, urls with mirror scheme, restrict mirror policy, mirror config map from Mirror)
func ApplyIndexConfigUrl(name string, origUrls []string, restrictMirror bool, mirrors map[string][]string) ([]string, error) {
	if repoConfig == nil {
		return nil, errors.New("load repo config first")
	}

	var newUrls []string
	if !restrictMirror {
		for _, mirror := range mirrors["ruyi-dist"] {
			u, err := url.JoinPath(mirror, name)
			if err != nil {
				return nil, err
			}

			slog.Debug("apply ruyi-dist url:", "url", u)
			newUrls = append(newUrls, u)
		}
	}

	for _, u := range origUrls {
		purl, err := url.Parse(u)
		if err != nil {
			return nil, err
		}

		scheme := purl.Scheme
		host := purl.Host
		path := purl.Path

		if scheme == "mirror" {
			rhost := mirrors[host]
			if rhost == nil || len(rhost) == 0 {
				return nil, errors.New("mirror not found in repo config: " + "mirror=" + host)
			}

			for _, rh := range rhost {
				ru, err := url.JoinPath(rh, path)
				if err != nil {
					return nil, err
				}
				slog.Debug("apply mirror url:", "mirror", host, "url", ru)
				newUrls = append(newUrls, ru)
			}
		} else {
			slog.Debug("apply origin url:", "url", u)
			newUrls = append(newUrls, u)
		}
	}

	return newUrls, nil
}

func LoadIndexData(repoPath string, dbTtlDays int64) error {
	if repoConfig == nil || repoPackages != nil || dbPackagesData == nil {
		packages, err := packagesIndexLoad(repoPath)
		if err != nil {
			return err
		}
		repoPackages = packages
	}

	// apply to database
	// refresh database before data expire
	ctx := context.Background()
	return db.AddViews(ctx, getRepoHash(indexRepoId), dbTtlDays, dbPackagesData)
}
