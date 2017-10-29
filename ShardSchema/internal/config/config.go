package config

import (
	"fmt"

	"github.com/pkg/errors"
	ini "gopkg.in/ini.v1"
)

type Config struct {
	Host             string
	Port             int
	User             string
	Password         string
	DBName           string
	ThrottlingFile   string
	MaxConcurrentDDL int
}

func LoadConfig(filename string) (*Config, error) {
	rawcfg, err := ini.InsensitiveLoad(filename)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("cannot load config file %q", filename))
	}

	cfg := &Config{
		Host:             rawcfg.Section("").Key("host").Value(),
		User:             rawcfg.Section("").Key("user").Value(),
		Password:         rawcfg.Section("").Key("password").Value(),
		ThrottlingFile:   rawcfg.Section("").Key("throttlingfile").Value(),
		DBName:           rawcfg.Section("").Key("dbname").Value(),
		Port:             3306,
		MaxConcurrentDDL: 2,
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("hostname cannot be empty")
	}

	if cfg.User == "" {
		return nil, fmt.Errorf("user cannot be empty")
	}

	if port, err := rawcfg.Section("").Key("port").Int(); err == nil {
		cfg.Port = port
	}

	if maxConcurrentDDL, err := rawcfg.Section("").Key("maxconcurrentddl").Int(); err == nil {
		cfg.MaxConcurrentDDL = maxConcurrentDDL
	}
	if cfg.MaxConcurrentDDL < 1 {
		cfg.MaxConcurrentDDL = 1
	}

	if cfg.ThrottlingFile == "" {
		cfg.ThrottlingFile = "/tmp/ShardSchema_throttle"
	}

	return cfg, nil
}
