package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"

	"github.com/adrg/xdg"
	"go.i3wm.org/i3/v4"
	"gopkg.in/yaml.v2"
)

// Runnder
// -------

type Runner interface {
	Run(t i3.EventType, event i3.Event) error
	Check() error
	String() string
}

// CommandRunner
// -------------

type CommandRunner struct {
	Command string `yaml:"cmd"`
}

func (r *CommandRunner) Run(t i3.EventType, e i3.Event) error {
	log.Printf("%s: run command: %s\n", t, r.Command)
	return nil
}

func (r *CommandRunner) Check() error {
	return nil
}

func (r *CommandRunner) String() string {
	return fmt.Sprintf(`CommandRunner "%s"`, r.Command)
}

// DirectoryRunner
// ---------------

type DirectoryRunner struct {
	Path string `yaml:"dir"`
}

func (r *DirectoryRunner) Run(t i3.EventType, e i3.Event) error {
	log.Printf("%s: run scripts in: %s\n", t, r.Path)
	return nil
}

func (r *DirectoryRunner) Check() error {
	if s, err := os.Stat(r.Path); err == nil && s.IsDir() {
		return nil
	} else if !s.IsDir() {
		return fmt.Errorf("not a directory: %s", r.Path)
	} else {
		return err
	}
}

func (r *DirectoryRunner) String() string {
	return fmt.Sprintf(`DirectoryRunner "%s"`, r.Path)
}

// Handler
// -------

type Handler struct {
	runner Runner
}

func (h *Handler) Run(t i3.EventType, e i3.Event) error {
	if !h.IsZero() {
		return h.runner.Run(t, e)
	}
	return nil
}

func (h *Handler) String() string {
	if !h.IsZero() {
		return h.runner.String()
	}
	return "Runner"
}

func (h *Handler) Check() error {
	if !h.IsZero() {
		return h.runner.Check()
	}
	return nil
}

func (h *Handler) IsZero() bool {
	return h.runner == nil
}

func (h *Handler) UnmarshalYAML(um func(interface{}) error) (err error) {
	m := make(map[string]interface{})

	if err = um(&m); err != nil {
		return
	}

	for key := range m {
		// Having 'cmd' case first means Command runners take
		// precedence over Directory runners in case a handler
		// contains both 'cmd' and 'dir' keys
		switch key {
		case "cmd":
			var r CommandRunner
			if err = um(&r); err == nil {
				h.runner = &r
			}
			break
		case "dir":
			var r DirectoryRunner
			if err = um(&r); err == nil {
				h.runner = &r
			}
			break
		}
	}

	if err != nil {
		err = h.Check()
	}

	return
}

// ConfigData
// ----------

type ConfigData struct {
	Events map[i3.EventType][]*Handler `yaml:"events"`
}

// Config
// ------

type Config struct {
	Path string     // Path to config file
	Data ConfigData // The actual config data
}

func (c *Config) String() string {
	return fmt.Sprintf("<Config %s>", c.Path)
}

// Init
// ----

func init() {
	log.SetFlags(0)
}

// Main
// ----

func main() {
	// get i3 version (also for checking if i3 is installed?)
	if ver, err := i3.GetVersion(); err != nil {
		log.Fatal(err)
	} else {
		fmt.Printf("i3: %s\n", ver.HumanReadable)
	}

	var (
		config      Config
		configFiles []string = []string{
			"i3watch.yml",
			"i3watch.yaml",
		}
	)

	if homeConfig, err := xdg.ConfigFile("i3watch/config.yml"); err == nil {
		configFiles = append(configFiles, homeConfig)
	}

	if data, path, err := readFirst(configFiles...); err != nil {
		log.Fatal(err)
	} else if cfg, err := readConfig(data); err != nil {
		log.Fatal(err)
	} else {
		config.Path = path
		config.Data = *cfg
		subscribe(&config)
	}
}

func readConfig(data []byte) (*ConfigData, error) {
	var config ConfigData

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func readFirst(files ...string) ([]byte, string, error) {
	for _, file := range files {
		if data, err := ioutil.ReadFile(file); err == nil {
			return data, file, nil
		}
	}
	return nil, "", nil
}

func subscribe(conf *Config) {
	for t, handlers := range conf.Data.Events {
		log.Printf("handlers: %s", t)

		for _, handler := range handlers {
			log.Printf("  %v", handler)
			go runListener(t, handler)
		}
	}

	log.Print("---")

	wait()
}

func runListener(t i3.EventType, h *Handler) {
	events := i3.Subscribe(t)

	for events.Next() {
		h.Run(t, events.Event())
	}

	log.Fatalf("i3 error: %s", events.Close())
}

func wait() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	s := <-c
	log.Printf("exit: %s", s)
}
