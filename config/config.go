package config

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/HannahMarsh/PrettyLogger"
	"github.com/ilyakaznacheev/cleanenv"
)

type BulletinBoard struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Address  string
	PromPort int `yaml:"promPort"`
}

type LastRegistered struct {
	Clients []Node `yaml:"clients"`
	Relays  []Node `yaml:"relays"`
}

type Node struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	PromPort int    `yaml:"promPort"`
}

type Config struct {
	MinimumClients          int           `yaml:"N"`
	MinimumRelays           int           `yaml:"n"`
	BulletinBoard           BulletinBoard `yaml:"bulletin_board"`
	Vis                     bool          `yaml:"vis"`
	PrometheusPath          string        `yaml:"prometheusPath"`
	ScrapeInterval          int           `yaml:"scrapeInterval"`
	ServerLoad              int           `yaml:"x"`
	D                       int           `yaml:"d"`
	Delta                   float64       `yaml:"delta"`
	Tao                     float64       `yaml:"tao"`
	L1                      int           `yaml:"l1"`
	L2                      int           `yaml:"l2"`
	Chi                     float64       `yaml:"chi"`
	DropAllOnionsFromClient int           `yaml:"dropAllOnionsFromClient"`
}

func GetVis() bool {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.Vis
}

func GetPrometheusPath() string {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.PrometheusPath
}

func GetBulletinBoardAddress() string {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.BulletinBoard.Address
}

func GetTao() float64 {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.Tao
}

func GetMinimumClients() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.MinimumClients
}

func GetMinimumRelays() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.MinimumRelays
}

func GetBulletinBoardHost() string {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.BulletinBoard.Host
}

func GetBulletinBoardPort() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.BulletinBoard.Port
}

func GetMetricsPort() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.BulletinBoard.PromPort
}

func GetMetricsUrl() string {
	mu.RLock()
	defer mu.RUnlock()
	return fmt.Sprintf("http://%s:%d", globalConfig.BulletinBoard.Host, globalConfig.BulletinBoard.PromPort)
}

func GetBulletinBoardUrl() string {
	mu.RLock()
	if globalConfig.BulletinBoard.Address != "" {
		defer mu.RUnlock()
		return globalConfig.BulletinBoard.Address
	}
	mu.RUnlock()
	mu.Lock()
	defer mu.Unlock()
	if globalConfig.BulletinBoard.Address == "" {
		globalConfig.BulletinBoard.Address = fmt.Sprintf("http://%s:%d", globalConfig.BulletinBoard.Host, globalConfig.BulletinBoard.Port)
	}
	return globalConfig.BulletinBoard.Address
}

func GetServerLoad() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.ServerLoad
}

func GetD() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.D
}

func GetDelta() float64 {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.Delta
}

func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return *globalConfig
}

func GetL1() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.L1
}

func GetL2() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.L2
}

func GetChi() float64 {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.Chi
}

func GetScrapeInterval() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.ScrapeInterval
}

func GetDropAllOnionsFromClient() int {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig.DropAllOnionsFromClient
}

var globalConfig *Config
var GlobalCtx context.Context
var GlobalCancel context.CancelFunc
var mu sync.RWMutex
var Path string

func InitGlobal() (error, string) {
	mu.Lock()
	defer mu.Unlock()
	GlobalCtx, GlobalCancel = context.WithCancel(context.Background())

	globalConfig = &Config{}

	path := ""

	if dir, err := os.Getwd(); err != nil {
		return PrettyLogger.WrapError(err, "config.NewConfig(): global config error"), ""
	} else if err2 := cleanenv.ReadConfig(dir+"/config/config.yml", globalConfig); err2 != nil {

		// Get the absolute path of the current file
		_, currentFile, _, ok := runtime.Caller(0)
		if !ok {
			return PrettyLogger.NewError("Failed to get current file path"), ""
		}
		currentDir := filepath.Dir(currentFile)
		configFilePath := filepath.Join(currentDir, "/config.yml")
		if err3 := cleanenv.ReadConfig(configFilePath, globalConfig); err3 != nil {
			return PrettyLogger.WrapError(err3, "config.NewConfig(): global config error"), ""
		} else {
			path = configFilePath
		}
	} else {
		path = dir + "/config/config.yml"
		if err3 := cleanenv.ReadEnv(globalConfig); err3 != nil {
			return PrettyLogger.WrapError(err3, "config.NewConfig(): global config error"), ""
		}
	}

	Path = strings.ReplaceAll(path, "config.yml", "lastRegisteredClientsRelays.yml")

	path = strings.ReplaceAll(path, "config.yml", "prometheus.yml")

	globalConfig.BulletinBoard.Address = fmt.Sprintf("http://%s:%d", globalConfig.BulletinBoard.Host, globalConfig.BulletinBoard.Port)

	return nil, path
}

func UpdateRegistered(clients []Node, relays []Node) {
	mu.Lock()
	defer mu.Unlock()

	registered := LastRegistered{
		Clients: clients,
		Relays:  relays,
	}

	// Marshal the struct into YAML format
	data, err := yaml.Marshal(&registered)
	if err != nil {
		slog.Error("failed to marshal prometheus config", err)
		return
	}

	// Open the file for writing (creates the file if it doesn't exist)
	file, err := os.OpenFile(Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		slog.Error("failed to open file", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			slog.Error("failed to close file", err)
		}
	}(file)

	// Write the YAML data to the file
	_, err = file.Write(data)
	if err != nil {
		slog.Error("failed to write prometheus config to file", err)
		return
	}

	// Ensure the data is flushed to disk immediately
	err = file.Sync()
	if err != nil {
		slog.Error("failed to flush prometheus config to disk", err)
	}

	slog.Info("registered clients and relays file updated", "path", Path)
}

func GetLastRegistered() (LastRegistered, error) {
	mu.RLock()
	defer mu.RUnlock()

	var registered LastRegistered

	file, err := os.Open(Path)
	if err != nil {
		return registered, PrettyLogger.WrapError(err, "failed to open file")
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			slog.Error("failed to close file", err)
		}
	}(file)

	// Decode the YAML file into a struct
	if err = yaml.NewDecoder(file).Decode(&registered); err != nil {
		return registered, PrettyLogger.WrapError(err, "failed to decode yaml")
	}

	return registered, nil
}

func UpdateConfig(cfg Config) {
	mu.Lock()
	defer mu.Unlock()
	if globalConfig == nil {
		globalConfig = &cfg
	}
	if cfg.BulletinBoard.Host != "" {
		globalConfig.BulletinBoard.Host = cfg.BulletinBoard.Host
	}
	if cfg.BulletinBoard.Port != 0 {
		globalConfig.BulletinBoard.Port = cfg.BulletinBoard.Port
	}
	if cfg.PrometheusPath != "" {
		globalConfig.PrometheusPath = cfg.PrometheusPath
	}
	if cfg.ScrapeInterval != 0 {
		globalConfig.ScrapeInterval = cfg.ScrapeInterval
	}
	if cfg.BulletinBoard.PromPort != 0 {
		globalConfig.BulletinBoard.PromPort = cfg.BulletinBoard.PromPort
	}
	if cfg.ServerLoad != 0 {
		globalConfig.ServerLoad = cfg.ServerLoad
	}
	if cfg.D != 0 {
		globalConfig.D = cfg.D
	}
	if cfg.Delta != 0 {
		globalConfig.Delta = cfg.Delta
	}
	if cfg.L1 != 0 {
		globalConfig.L1 = cfg.L1
	}
	if cfg.L2 != 0 {
		globalConfig.L2 = cfg.L2
	}
	if cfg.Chi != 0 {
		globalConfig.Chi = cfg.Chi
	}
	if cfg.DropAllOnionsFromClient != 0 {
		globalConfig.DropAllOnionsFromClient = cfg.DropAllOnionsFromClient
	}
	if cfg.MinimumClients != 0 {
		globalConfig.MinimumClients = cfg.MinimumClients
	}
	if cfg.MinimumRelays != 0 {
		globalConfig.MinimumRelays = cfg.MinimumRelays
	}
	globalConfig.BulletinBoard.Address = fmt.Sprintf("http://%s:%d", globalConfig.BulletinBoard.Host, globalConfig.BulletinBoard.Port)
}
