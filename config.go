package main

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

func InitConfig(configName string, Context *_context) error {
	viper.SetConfigName(configName) // name of config file (without extension)

	viper.SetConfigType("yaml") // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")    // optionally look for config in the working directory
	viper.AutomaticEnv()        // read in environment variables that match

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file, %s", err)
	}
	if err := viper.Unmarshal(&Context.Config); err != nil {
		return fmt.Errorf("error parsing config: %s", err)
	}

	// -----------------
	if Context.Config.QueueSize < 1 {
		Context.Config.QueueSize = 1
	}
	if Context.Config.Workers < 1 {
		Context.Config.Workers = 1
	}

	// -----------------
	Context.InputTimeout = time.Duration(viper.GetUint32("input_timeout"))
	if Context.InputTimeout <= 0 {
		Context.InputTimeout = 1000
	}
	Context.InputTimeout *= time.Millisecond

	Context.OutputTimeout = time.Duration(viper.GetUint32("output_timeout"))
	if Context.OutputTimeout <= 0 {
		Context.OutputTimeout = 1000
	}
	Context.OutputTimeout *= time.Millisecond

	Context.ExecTimeout = time.Duration(viper.GetUint32("exec_timeout"))
	if Context.ExecTimeout <= 0 {
		Context.ExecTimeout = 1000
	}
	Context.ExecTimeout *= time.Millisecond

	Context.Messages = make(chan string, Context.Config.QueueSize)
	Context.StopChan = make(chan bool)

	return nil
}
