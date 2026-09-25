package testbot

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/repo"
	"github.com/ruyisdk-test/ruyi-index-test-bot/bot/web"
	"go.yaml.in/yaml/v3"
)

const filename = "config.yaml"

type Config struct {
	Server struct {
		ListenAddr        string `yaml:"listen_addr" json:"-"`
		ControlListenAddr string `yaml:"control_addr" json:"-"`
	} `yaml:"server" json:"-"`

	ResolveBot struct {
		Url       string `yaml:"control_url" json:"-"`
		Available bool   `yaml:"-" json:"-"`
		Version   string `yaml:"-" json:"version"`
	} `yaml:"resolve_bot" json:"-"`

	Model struct {
		Provider  string `yaml:"provider"`
		BaseUrl   string `yaml:"base_url"`
		ApiKey    string `yaml:"api_key" json:"-"`
		ModelName string `yaml:"model"`
	} `yaml:"model" json:"-"`

	Github struct {
		Pat string `yaml:"token" json:"-"`
	} `yaml:"github" json:"-"`

	Cache struct {
		CacheDir string `yaml:"cache_dir" json:"-"`
	} `yaml:"cache" json:"-"`

	Repo     repo.Config `yaml:"repo" json:"repo"`
	Upstream repo.Config `yaml:"upstream" json:"upstream"`

	Valkey struct {
		Addr    string `yaml:"addr" json:"addr"`
		DataTtl int64  `yaml:"data_ttl" json:"data_ttl"`
	} `yaml:"valkey" json:"valkey"`

	BeforeServe Cmd          `yaml:"before_serve" json:"-"`
	Cron        []CronConfig `yaml:"cron" json:"-"`

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
		slog.Warn("config file not found")
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

	if config.Server.ControlListenAddr == "" {
		config.Server.ControlListenAddr = "127.0.0.1:9876"
	}
	if config.Server.ListenAddr == "" {
		config.Server.ListenAddr = "127.0.0.1:9877"
	}
	if config.ResolveBot.Url == "" {
		config.ResolveBot.Url = "http://127.0.0.1:9874"
	}
	if config.Cache.CacheDir == "" {
		config.Cache.CacheDir = filepath.Join(pathCurrent, "cache")
	}
	config.Repo.CacheDir = path.Join(config.Cache.CacheDir, "repo", "ruyisdk")
	config.Upstream.CacheDir = path.Join(config.Cache.CacheDir, "upstream", "ruyisdk-test")
	if config.Repo.Remote == "" {
		config.Repo.Remote = "https://github.com/ruyisdk/packages-index.git"
	}
	if config.Repo.Branch == "" {
		config.Repo.Branch = "main"
	}
	if config.Upstream.Remote == "" {
		config.Upstream.Remote = "https://github.com/ruyisdk-test/ruyi-index-upstreams-index.git"
	}
	if config.Upstream.Branch == "" {
		config.Upstream.Branch = "main"
	}
	if config.Valkey.Addr == "" {
		config.Valkey.Addr = "127.0.0.1:6379"
	}
	if config.Valkey.DataTtl == 0 {
		config.Valkey.DataTtl = 7 // days
	}
	if config.Github.Pat == "" && !config.configInit {
		return nil, errors.New("github pat not configured")
	}
	if config.Model.ApiKey == "" && !config.configInit {
		return nil, errors.New("model apikey not configured")
	}
	if config.Model.ModelName == "" {
		config.Model.ModelName = "gpt-4o"
	}
	if config.Model.Provider == "" {
		config.Model.Provider = "unknown"
	}
	if config.Model.BaseUrl == "" {
		config.Model.BaseUrl = "https://llalla.iscas.ac.cn/v9/"
	}

	if _, err := os.Stat(config.Cache.CacheDir); os.IsNotExist(err) {
		if err := os.Mkdir(config.Cache.CacheDir, 0755); err != nil {
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

		return nil, errors.New("edit init config file before next run")
	}

	slog.Info("test github pat valid")
	err = pingGithubApi(&config)
	if err != nil {
		return nil, err
	}
	slog.Info("test model settings valid")
	err = pingModelHello(&config)
	if err != nil {
		return nil, err
	}

	slog.Info("service listening on address:", "addr", config.Server.ListenAddr)
	slog.Info("control listening on address:", "addr", config.Server.ControlListenAddr)
	if err = pingResolveBot(&config); err != nil {
		slog.Warn("pinging resolve bot failed:", "url", config.ResolveBot.Url)
		slog.Info("run without resolve bot")
		config.ResolveBot.Available = false
	} else {
		slog.Info("use resolve bot:", "url", config.ResolveBot.Url)
		slog.Info("resolve bot version:", "version", config.ResolveBot.Version)
		config.ResolveBot.Available = true
	}
	slog.Info("use cache dir:", "path", config.Cache.CacheDir)
	slog.Info("connect valkey address:", "addr", config.Valkey.Addr)
	slog.Info("connect valkey data TTL:", "days", config.Valkey.DataTtl)

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

func pingGithubApi(config *Config) error {
	if config.Github.Pat == "" {
		return errors.New("no github pat configured")
	}

	err := web.InitGithubClient(config.Github.Pat)
	if err != nil {
		return err
	}

	return web.ListFoxOrgs()
}

func pingModelHello(config *Config) error {
	return ModelHello(config)
}

func pingResolveBot(config *Config) error {
	u, err := url.JoinPath(config.ResolveBot.Url, "/version")
	if err != nil {
		return err
	}
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("bad statuc code from resolveBot: " + resp.Status)
	}

	buf := make([]byte, resp.ContentLength)
	_, err = resp.Body.Read(buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && err != io.EOF {
		return err
	}

	err = json.Unmarshal(buf, &(config.ResolveBot))
	if err != nil {
		return err
	}

	return nil
}
