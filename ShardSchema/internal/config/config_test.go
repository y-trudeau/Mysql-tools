package config

import (
	"testing"

	tu "github.com/y-trudeau/Mysql-tools/ShardSchema/testutils"
)

func TestDefaultValues(t *testing.T) {
	cfg, err := LoadConfig("./testdata/config01.ini")
	tu.Ok(t, err)

	want := &Config{
		Host:             "localhost",
		Port:             3306,
		User:             "root",
		Password:         "",
		ThrottlingFile:   "/tmp/ShardSchema_throttle",
		DBName:           "",
		MaxConcurrentDDL: 2,
	}
	tu.Equals(t, cfg, want)
}

func TestRequiredValues01(t *testing.T) {
	cfg, err := LoadConfig("./testdata/config02.ini")
	tu.NotOk(t, err)
	tu.Assert(t, cfg == nil, "on errors, config should be nil")
}

func TestRequiredValues02(t *testing.T) {
	cfg, err := LoadConfig("./testdata/config03.ini")
	tu.NotOk(t, err)
	tu.Assert(t, cfg == nil, "on errors, config should be nil")
}
