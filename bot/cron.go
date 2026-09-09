package bot

import (
	"time"

	"github.com/robfig/cron/v3"
)

type CronConfig struct {
	Cron   string            `yaml:"cron"`
	Cmd    string            `yaml:"cmd"`
	Params map[string]string `yaml:"params"`

	entryID cron.EntryID `yaml:"-"`
}

func CronStart(cfg *Config) (*cron.Cron, error) {
	c := cron.New()

	for _, conf := range cfg.Cron {
		if err := CmdVerify(conf.Cmd, conf.Params); err != nil {
			return nil, err
		}
		id, err := c.AddFunc(conf.Cron, func() { CmdRun(conf.Cmd, conf.Params) })

		if err != nil {
			return nil, err
		}
		conf.entryID = id
	}

	c.Start()
	return c, nil
}

func CronStop(c *cron.Cron) {
	c.Stop()
}

type CronEntryInfo struct {
	Id   cron.EntryID `json:"id"`
	Prev *time.Time   `json:"prev"`
	Next *time.Time   `json:"next"`
}

func GetCronEntryInfo(c *cron.Cron) []CronEntryInfo {
	entries := c.Entries()
	result := make([]CronEntryInfo, 0, len(entries))

	for _, entry := range entries {
		var prev *time.Time
		var next *time.Time

		if !entry.Prev.IsZero() {
			t := entry.Prev
			prev = &t
		}

		if !entry.Next.IsZero() {
			t := entry.Next
			next = &t
		}

		result = append(result, CronEntryInfo{
			Id:   entry.ID,
			Prev: prev,
			Next: next,
		})
	}

	return result
}
