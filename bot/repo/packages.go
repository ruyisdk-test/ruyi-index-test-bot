package repo

import (
	"context"
	"errors"
	"log/slog"
	url2 "net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/db"
)

type Config struct {
	RepoVersion string `toml:"ruyi-repo"`
	RepoId      string `toml:"repo.id"`
	RepoDoc     string `toml:"repo.doc_uri"`
	RepoName    string `toml:"repo.name"`

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
	VendorName  string `toml:"vendor.name"`
	VendorEula  string `toml:"vendor.eula"`
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

	InstallSize int64 `toml:"metadata.install_size"`
}

type VersionChecksum struct {
	Sha256 string `toml:"sha256"`
	Sha512 string `toml:"sha512"`
}

func configLoad(repoPath string) (*Config, error) {
	configPath := filepath.Join(repoPath, "config.toml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := Config{}
	if err = toml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

var repoConfig *Config = nil
var repoPackages []PackageGroups = nil

func packagesLoad(repoPath string, dbTtlDays int64) ([]PackageGroups, error) {
	config, err := configLoad(repoPath)
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
				dbData[entry.Name()][pkgentry.Name()][vName] = db.VersionData{
					Distfiles: make(map[string][]string),
				}

				data, err := os.ReadFile(filepath.Join(packagesPath, entry.Name(), pkgentry.Name(), verentry.Name()))
				if err != nil {
					return nil, err
				}

				if err = toml.Unmarshal(data, &groups[i].Packages[j].Versions[k]); err != nil {
					return nil, err
				}

				for _, distfile := range groups[i].Packages[j].Versions[k].Distfiles {
					rm := slices.Contains(distfile.Restrict, "mirror")
					newUrls, err := applyConfigUrl(distfile.Name, distfile.Urls, rm, repoMirrors)
					if err != nil {
						return nil, err
					}

					distfile.Urls = newUrls
					dbData[entry.Name()][pkgentry.Name()][vName].Distfiles[distfile.Name] = newUrls
				}
			}
		}
	}

	// apply to database
	ctx := context.Background()

	return groups, db.AddViews(ctx, getRepoHash(), dbTtlDays, dbData)
}

func applyConfigUrl(name string, origUrls []string, restrictMirror bool, mirrors map[string][]string) ([]string, error) {
	if repoConfig == nil {
		return nil, errors.New("load repo config first")
	}

	var newUrls []string
	if !restrictMirror {
		for _, mirror := range mirrors["ruyi-dist"] {
			url, err := url2.JoinPath(mirror, name)
			if err != nil {
				return nil, err
			}

			slog.Debug("apply ruyi-dist url:", "url", url)
			newUrls = append(newUrls, url)
		}
	}

	for _, url := range origUrls {
		purl, err := url2.Parse(url)
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
				ru, err := url2.JoinPath(rh, path)
				if err != nil {
					return nil, err
				}
				slog.Debug("apply mirror url:", "mirror", host, "url", ru)
				newUrls = append(newUrls, ru)
			}
		} else {
			slog.Debug("apply origin url:", "url", url)
			newUrls = append(newUrls, url)
		}
	}

	return newUrls, nil
}

func LoadData(repoPath string, dbTtlDays int64) error {

	packages, err := packagesLoad(repoPath, dbTtlDays)
	if err != nil {
		return err
	}
	repoPackages = packages

	return nil
}
