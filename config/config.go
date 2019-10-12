package config

import (
	"bytes"
	"errors"
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/helloyi/goastch/galang/gen"
	"github.com/helloyi/goastch/galang/parser"
	"github.com/helloyi/goastch/goastcher"
)

type (
	// Config ...
	Config struct {
		Choke      int
		Root       string
		Path       string
		ModulePath string
		Build      *build.Context
		Enabled    map[string]*Checker
		Ignored    map[string]bool
		Override   map[string]map[string]*Checker
		Checkers   map[string]*Checker
	}

	// Checker ...
	Checker struct {
		Ger     goastcher.Goastcher
		Message string
	}
)

// New ...
func New() (*Config, error) {
	viper.SetConfigName("govet")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	conf := &Config{
		Build: &build.Default,
	}
	if err := conf.parse(); err != nil {
		return nil, err
	}

	return conf, nil
}

func newFromString(s string) (*Config, error) {
	viper.SetConfigType("toml")
	rawConf := strings.NewReader(s)
	if err := viper.ReadConfig(rawConf); err != nil {
		return nil, err
	}
	conf := &Config{
		Build: &build.Default,
	}
	if err := conf.parse(); err != nil {
		return nil, err
	}
	return conf, nil
}

func (c *Config) parse() error {
	if err := c.parseBasic(); err != nil {
		return err
	}
	if err := c.parseIgnored(); err != nil {
		return err
	}
	if err := c.parseEnabled(); err != nil {
		return err
	}
	return c.parseOverride()
}

func modulePath(mod []byte) string {
	idx := bytes.Index(mod, []byte("module"))
	if idx < 0 {
		return ""
	}
	bs := mod[idx:]
	idx = bytes.Index(bs, []byte("\n"))
	if idx < 0 {
		return ""
	}
	bfs := bytes.Fields(bs[:idx])
	return string(bfs[1])
}

func (c *Config) parseBasic() error {
	c.Choke = viper.GetInt("choke")
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if c.Build == nil {
		c.Build = &build.Default
	}

	modPath := filepath.Join(wd, "go.mod")
	_, err = os.Stat(modPath)
	isExistMod := (err == nil)

	var mpath string
	switch os.Getenv("GO111MODULE") {
	case "on":
		if !isExistMod {
			return fmt.Errorf("not exist go.mod")
		}

		mod, err := ioutil.ReadFile(modPath)
		if err != nil {
			return err
		}
		mpath = modulePath(mod)

	case "off":
		mpath = ""

	case "auto":
		fallthrough
	default:
		if isExistMod {
			mod, err := ioutil.ReadFile(modPath)
			if err != nil {
				return err
			}
			mpath = modulePath(mod)
		} else {
			mpath = ""
		}
	}

	c.ModulePath = mpath
	c.Path = wd

	checkers := make(map[string]*Checker, len(c.Checkers))
	var checkersConf []struct {
		Name    string `mapstructure:"name"`
		Goastch string `mapstructure:"goastch"`
		Message string `mapstructure:"message"`
	}
	if err := viper.UnmarshalKey("checkers", &checkersConf); err != nil {
		return err
	}
	for _, checkerConf := range checkersConf {
		if _, have := checkers[checkerConf.Name]; have {
			return errors.New("Duplicate checker")
		}
		node, err := parser.ParseGer(checkerConf.Goastch)
		if err != nil {
			return err
		}

		ger, err := gen.Ger(node)
		if err != nil {
			return err
		}
		checkers[checkerConf.Name] = &Checker{
			Ger:     ger,
			Message: checkerConf.Message,
		}
	}
	c.Checkers = checkers

	return nil
}

func (c *Config) parseIgnored() error {
	c.Ignored = make(map[string]bool)
	for _, path := range viper.GetStringSlice("ignored") {
		if filepath.IsAbs(path) {
			return fmt.Errorf("%s", path)
		}
		subPath := strings.TrimPrefix(path, c.ModulePath)
		absPath := filepath.Join(c.Path, subPath)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return fmt.Errorf("File does not exist %s", absPath)
		}

		c.Ignored[absPath] = true
	}
	return nil
}

func (c *Config) parseEnabled() error {
	enabledConf := viper.GetStringSlice("enabled")
	disabledConf := viper.GetStringSlice("disabled")
	if enabledConf != nil && disabledConf != nil {
		return errors.New("Only one can be used for 'enabled' and 'disabled'")
	}

	enabled := make(map[string]*Checker)
	for _, name := range enabledConf {
		checker := c.Checkers[name]
		if checker == nil {
			return fmt.Errorf("'%s' checker not exist", name)
		}
		enabled[name] = checker
	}

	if disabledConf != nil {
		disabled := make(map[string]bool)
		for _, name := range disabledConf {
			disabled[name] = true
		}
		for name, checker := range c.Checkers {
			if disabled[name] {
				continue
			}
			enabled[name] = checker
		}
	} else {
		for name, checker := range c.Checkers {
			enabled[name] = checker
		}
	}
	c.Enabled = enabled
	return nil
}

func (c *Config) parseOverride() error {
	var override []struct {
		Package  string   `mapstructure:"package"`
		File     string   `mapstructure:"file"`
		Enabled  []string `mapstructure:"enabled"`
		Disabled []string `mapstructure:"disabled"`
	}
	if err := viper.UnmarshalKey("override", &override); err != nil {
		return err
	}
	for _, check := range override {
		if check.Enabled != nil && check.Disabled != nil {
			return errors.New("Only one can be used for 'enabled' and 'disabled'")
		}

		if check.Package != "" && check.File != "" {
			return errors.New("Only one can be used for 'package' and 'file'")
		}

		fname := ""
		if check.Package != "" {
			fname = check.Package
		} else if check.File != "" {
			fname = check.File
		} else {
			return errors.New("Requred 'package' or 'file' for a override")
		}
		p := strings.TrimPrefix(fname, "github.com/helloyi/")
		path := filepath.Join(c.Root, p)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return errors.New("Path does not exist")
		}

		enabled := make(map[string]*Checker)
		if check.Enabled != nil {
			for _, name := range check.Enabled {
				checker := c.Checkers[name]
				if checker == nil {
					return fmt.Errorf("")
				}
				enabled[name] = checker
			}
		} else {
			disabled := make(map[string]bool)
			for _, name := range check.Disabled {
				disabled[name] = true
			}
			for name, checker := range c.Checkers {
				if disabled[name] {
					continue
				}
				enabled[name] = checker
			}
		}
		c.Override[path] = enabled
	}

	return nil
}
