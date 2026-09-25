package repo

type Config struct {
	Remote   string `yaml:"remote" json:"remote"`
	Branch   string `yaml:"branch" json:"branch"`
	CacheDir string `yaml:"-" json:"cache_dir"`
}

func Update(cfgIndex *Config, cfgUpstream *Config, dataTtl int64) error {
	err := CheckLatest(indexRepoId, cfgIndex.CacheDir, cfgIndex.Remote, cfgIndex.Branch)
	if err != nil {
		return err
	}
	err = CheckLatest(upstreamRepoId, cfgUpstream.CacheDir, cfgUpstream.Remote, cfgUpstream.Branch)
	if err != nil {
		return err
	}

	err = LoadIndexData(cfgIndex.CacheDir, dataTtl)
	if err != nil {
		return err
	}
	return UpstreamLoad(cfgUpstream.CacheDir)
}
