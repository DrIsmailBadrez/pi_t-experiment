package metrics

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	pl "github.com/HannahMarsh/PrettyLogger"
	"github.com/HannahMarsh/pi_t-experiment/config"
	"github.com/HannahMarsh/pi_t-experiment/internal/api/structs"
	"github.com/ilyakaznacheev/cleanenv"
	"gopkg.in/yaml.v3"
)

type Global struct {
	ScrapeInterval string         `yaml:"scrape_interval"`
	ExternalLabels ExternalLabels `yaml:"external_labels"`
}

type ExternalLabels struct {
	Monitor string `yaml:"monitor"`
}

type ScrapeConfig struct {
	JobName        string         `yaml:"job_name"`
	ScrapeInterval string         `yaml:"scrape_interval"`
	StaticConfigs  []StaticConfig `yaml:"static_configs"`
}

type StaticConfig struct {
	Targets []string `yaml:"targets"`
}

type PromConfig struct {
	Global        Global         `yaml:"global"`
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs"`
	RuleFile      []string       `yaml:"rule_files"`
}

var PID int
var mu = &sync.Mutex{}

func RestartPrometheus(relays, clients []structs.PublicNodeApi) error {
	path := ""

	// Debug output for clients
	fmt.Printf("Number of clients: %d\n", len(clients))
	for _, client := range clients {
		fmt.Printf("Client: ID: %d, Host: %s, Port: %d\n", client.ID, client.Host, client.PrometheusPort)
	}

	// Debug output for relays
	fmt.Printf("Number of relays: %d\n", len(relays))
	for _, relay := range relays {
		fmt.Printf("Relay: ID: %d, Host: %s, Port: %d\n", relay.ID, relay.Host, relay.PrometheusPort)
	}

	promCfg := &PromConfig{}

	if dir, err := os.Getwd(); err != nil {
		return pl.WrapError(err, "config.NewConfig(): global config error")
	} else if err2 := cleanenv.ReadConfig(dir+"/internal/model/bulletin_board/metrics/prometheus.yml", promCfg); err2 != nil {
		_, currentFile, _, ok := runtime.Caller(0)
		if !ok {
			return pl.NewError("Failed to get current file path")
		}
		currentDir := filepath.Dir(currentFile)
		configFilePath := filepath.Join(currentDir, "/prometheus.yml")
		if err3 := cleanenv.ReadConfig(configFilePath, promCfg); err3 != nil {
			return pl.WrapError(err3, "InitPrometheusConfig(): global config error")
		} else {
			path = configFilePath
		}
	} else {
		path = dir + "/internal/model/bulletin_board/metrics/prometheus.yml"
		if err3 := cleanenv.ReadEnv(promCfg); err3 != nil {
			return pl.WrapError(err3, "InitPrometheusConfig(): global config error")
		}
	}

	rules := strings.Replace(path, "prometheus.yml", "rules.yml", 1)

	promCfg_ := PromConfig{
		Global: Global{
			ScrapeInterval: fmt.Sprintf("%ds", config.GetScrapeInterval()),
			ExternalLabels: ExternalLabels{
				Monitor: "pi_t",
			},
		},
		ScrapeConfigs: []ScrapeConfig{},
		RuleFile:      []string{rules},
	}

	for _, client := range clients {
		target := fmt.Sprintf("%s:%d", client.Host, client.PrometheusPort)
		promCfg_.ScrapeConfigs = append(promCfg_.ScrapeConfigs, ScrapeConfig{
			JobName:        fmt.Sprintf("client-%d", client.ID),
			ScrapeInterval: "5s",
			StaticConfigs: []StaticConfig{
				{
					Targets: []string{target},
				},
			},
		})
		// log added scrape target
		fmt.Printf("Added a Client scraper for IP: %s and port: %d\n", client.Host, client.PrometheusPort)
	}

	for _, relay := range relays {
		target := fmt.Sprintf("%s:%d", relay.Host, relay.PrometheusPort)
		promCfg_.ScrapeConfigs = append(promCfg_.ScrapeConfigs, ScrapeConfig{
			JobName:        fmt.Sprintf("relay-%d", relay.ID),
			ScrapeInterval: "5s",
			StaticConfigs: []StaticConfig{
				{
					Targets: []string{target},
				},
			},
		})
		// log added scrape target
		fmt.Printf("Added a Relay scraper for IP: %s and port: %d\n", relay.Host, relay.PrometheusPort)
	}

	// Marshal the struct into YAML format
	data, err := yaml.Marshal(&promCfg_)
	if err != nil {
		return pl.WrapError(err, "failed to marshal prometheus config")
	}

	// Open the file for writing (creates the file if it doesn't exist)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return pl.WrapError(err, "failed to open file for writing")
	}
	defer file.Close()

	// Write the YAML data to the file
	_, err = file.Write(data)
	if err != nil {
		return pl.WrapError(err, "failed to write prometheus config to file")
	}

	// Ensure the data is flushed to disk immediately
	err = file.Sync()
	if err != nil {
		return pl.WrapError(err, "failed to flush prometheus config to disk")
	}

	slog.Info("prometheus config written to file", "path", path)

	// rint the content of the config file
	content, err := os.ReadFile(path)
	if err != nil {
		return pl.WrapError(err, "failed to read prometheus config file")
	}
	fmt.Printf("Prometheus Config File Content:\n%s\n", content)

	// Stop Prometheus
	if err := StopPrometheus(); err != nil {
		return pl.WrapError(err, "failed to stop Prometheus")
	}

	// Start Prometheus
	if err := StartPrometheus(path); err != nil {
		return pl.WrapError(err, "failed to start Prometheus")
	}

	slog.Info("Prometheus restarted successfully", "pid", PID)

	return nil
}

func StartPrometheus(path string) error {

	mu.Lock()
	defer mu.Unlock()
	// Start Prometheus

	// Command to start Prometheus
	cmd := exec.Command(config.GetPrometheusPath(), "--config.file", path)

	slog.Info("Starting prometheus with config", "path", path)

	// Set the environment variables, if needed
	cmd.Env = os.Environ()

	// Set the command's standard output and error to the current process's output and error
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the Prometheus process
	err := cmd.Start()
	if err != nil {
		slog.Error("failed to start Prometheus", err)
		os.Exit(1)
	}

	PID = cmd.Process.Pid
	return nil
}

func StopPrometheus() error {
	// Sop Prometheus

	mu.Lock()
	defer mu.Unlock()
	if PID != 0 {
		cmdStop := exec.Command("kill", fmt.Sprintf("%d", PID))
		err := cmdStop.Run()
		if err != nil {
			slog.Error("failed to stop Prometheus", err)
			return pl.WrapError(err, "failed to stop Prometheus")
		} else {
			slog.Info("successfully stopped Prometheus")
			PID = 0
		}
	} else {
		slog.Info("No running Prometheus instance found, skipping stop")
	}

	return nil
}
