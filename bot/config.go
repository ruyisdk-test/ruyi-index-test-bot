package bot

import (
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const filename = "config.yaml"

type Config struct {
	ListenAddr string `yaml:"server.listen_addr"`

	CacheDir string `yaml:"cache.cache_dir"`

	RepoRemote   string `yaml:"repo.remote"`
	RepoBranch   string `yaml:"repo.branch"`
	repoCacheDir string `yaml:"-"`

	ValkeyAddr string `yaml:"valkey.addr"`

	BeforeServe Cmd          `yaml:"before_serve"`
	Cron        []CronConfig `yaml:"cron"`

	configPath string     `yaml:"-"`
	configInit bool       `yaml:"-"`
	node       *yaml.Node `yaml:"-"`
}

func CfgLoad() (*Config, error) {
	pathCurrent, err := filepath.Abs(".")
	if err != nil {
		return nil, err
	}
	pathConfig := filepath.Join(pathCurrent, filename)

	var config Config
	config.configPath = pathConfig

	if _, err := os.Stat(pathConfig); os.IsNotExist(err) {
		config.configInit = true
		slog.Warn("config file not found, using defaults")
	} else {

		slog.Info("loading config file:", "path", config.configPath)

		var data []byte
		data, err = os.ReadFile(pathConfig)
		if err != nil {
			return nil, err
		}

		var node yaml.Node
		if err = yaml.Unmarshal(data, &node); err != nil {
			slog.Error("error parsing config file", "error", err)
			return nil, err
		}

		if err = node.Decode(&config); err != nil {
			slog.Error("error decode content", "error", err)
			return nil, err
		}

		config.node = &node
	}

	if config.ListenAddr == "" {
		config.ListenAddr = "127.0.0.1:9876"
	}
	if config.CacheDir == "" {
		config.CacheDir = filepath.Join(pathCurrent, "cache")
	}
	config.repoCacheDir = path.Join(config.CacheDir, "repo", "ruyisdk")
	if config.RepoRemote == "" {
		config.RepoRemote = "https://github.com/ruyisdk/packages-index.git"
	}
	if config.RepoBranch == "" {
		config.RepoBranch = "main"
	}
	if config.ValkeyAddr == "" {
		config.ValkeyAddr = "127.0.0.1:6379"
	}

	slog.Info("listening on address:", "addr", config.ListenAddr)
	slog.Info("use cache dir:", "path", config.CacheDir)
	slog.Info("connect valkey address:", "addr", config.ValkeyAddr)

	if _, err := os.Stat(config.CacheDir); os.IsNotExist(err) {
		if err := os.Mkdir(config.CacheDir, 0755); err != nil {
			slog.Error("error creating cache dir", "error", err)
			return nil, err
		}
	}

	// save config here for we have CfgSave TODOs
	// when we solve that TODO, we can save all config on bot exit
	if config.configInit {
		if err = CfgSave(&config); err != nil {
			slog.Warn("error saving init config", "error", err)
		}
	}

	return &config, nil
}

// CfgSave save bot config
// TODO: save without damage file format
func CfgSave(config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(config.configPath, data, 0644)
}
