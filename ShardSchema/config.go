package main 

import (
    "os"
	"bufio"
	"strings"
)

type Config struct {
    ConfigValues  map[string]string       // Precomputed cache of option name => value
}

var cfg Config


func getConfig() *Config {
	return &cfg
}

func (cfg *Config) LoadConfig (configFilePath string) {
    
    if len(configFilePath) == 0 {
        configFilePath = "/etc/ShardSchema.cnf"
    }

	if _, err := os.Stat(configFilePath); !os.IsNotExist(err) {
		Logger.Println("config file exists")

		file, err := os.Open(configFilePath)
		defer file.Close()
		Logger.Println("config file opened")

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			Logger.Println("read line: " + line)

			// See if there is a "="
			if strings.Count(line,"=") > 0 {
				Logger.Println("found =")
				pos := strings.Index(line,"=")
				varName := line[0:pos]
				varValue := line[pos+1:]
				Logger.Println("varName: " + varName + ", varValue: " + varValue)
                cfg.SetValue(varName,varValue);
			}
		}
	}
}

func IsValidName (name string) bool {
    
    ValidNames := [...]string{"dbDSN","throttlingFile","MaxConcurrentDDL"}

    for _, vItem := range ValidNames {
        if vItem == name {
            return true
        }
    }
    return false
}


func (cfg *Config) SetValue (name string, value string) {
        if IsValidName(name) {
            cfg.ConfigValues[name] = value
        } else {
            Logger.Println("Ignoring variable name " + name)
        }

}


func (cfg *Config) SetDefaultValues () {
    cfg.SetValue("dbDNS","skeema:skeema@tcp(10.0.3.87:3306)/shardschema")
    cfg.SetValue("throttlingFile","/tmp/ShardSchema_throttle")
    cfg.SetValue("MaxConcurrentDDL","2")
}

func (cfg *Config) GetValue(name string) string {
    return cfg.ConfigValues[name]
}

func (cfg *Config) InitConfig(configFilePath string) {
	cfg.ConfigValues = make(map[string]string)
	cfg.SetDefaultValues()
	cfg.LoadConfig(configFilePath)
}
