package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/get-woke/woke/pkg/rule"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// Config contains a list of rules
type Config struct {
	Rules              []*rule.Rule `yaml:"rules"`
	IgnoreFiles        []string     `yaml:"ignore_files"`
	SuccessExitMessage *string      `yaml:"success_exit_message"`
	IncludeNote        bool         `yaml:"include_note"`
	ExcludeCategories  []string     `yaml:"exclude_categories"`
}

// NewConfig returns a new Config
func NewConfig(filename string) (*Config, error) {
	var c Config
	if len(filename) > 0 {
		var err error
		c, err = loadConfig(filename)
		if err != nil {
			return nil, err
		}

		log.Debug().Str("config", filename).Msg("loaded config file")
		logRuleset("config", c.Rules)

		// Ignore the config filename, it will always match on its own rules
		c.IgnoreFiles = append(c.IgnoreFiles, relative(filename))
	} else {
		log.Debug().Msg("no config file loaded, using only default rules")
	}

	c.ConfigureRules()
	logRuleset("all enabled", c.Rules)

	return &c, nil
}

func (c *Config) GetSuccessExitMessage() string {
	if c.SuccessExitMessage == nil {
		return "No findings found."
	}
	return *c.SuccessExitMessage
}

func (c *Config) inExistingRules(r *rule.Rule) bool {
	for _, n := range c.Rules {
		if n.Name == r.Name {
			return true
		}
	}
	return false
}

// ConfigureRules adds the config Rules to DefaultRules
// Configure RegExps for all rules
// Configure IncludeNote for all rules
func (c *Config) ConfigureRules() {
	for _, r := range rule.DefaultRules {
		if !c.inExistingRules(r) {
			c.Rules = append(c.Rules, r)
		}
	}

	logRuleset("default", rule.DefaultRules)
	var excludedIndices []int

RuleLoop:
	for i, r := range c.Rules {
		// If any of a rule's categories are in ExcludedCategories, remove the rule
		for _, ex := range c.ExcludeCategories {
			if r.ContainsCategory(ex) {
				excludedIndices = append(excludedIndices, i)
				continue RuleLoop
			}
		}

		r.SetRegexp()
		r.SetIncludeNote(c.IncludeNote)
	}

	// Remove excluded rules after done iterating through them
	if len(c.ExcludeCategories) > 0 {
		log.Debug().Strs("categories", c.ExcludeCategories).Msg("excluding categories")
	}
	for _, exIdx := range excludedIndices {
		log.Debug().Strs("categories", c.Rules[exIdx].Options.Categories).Msg(fmt.Sprintf("rule \"%s\" excluded with categories", c.Rules[exIdx].Name))
		c.RemoveRule(exIdx)
	}
}

// Remove a rule at index i while maintaining order
func (c *Config) RemoveRule(i int) {
	if i >= len(c.Rules) || i < 0 {
		return
	}
	c.Rules = append(c.Rules[:i], c.Rules[i+1:]...)
}

func loadConfig(filename string) (c Config, err error) {
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return c, err
	}

	return c, yaml.Unmarshal(yamlFile, &c)
}

func relative(filename string) string {
	// viper provides an absolute path to the config file, but we want the relative
	// path to the config file from the current directory to make it easy for woke to ignore it
	if filepath.IsAbs(filename) {
		cwd, _ := os.Getwd()
		if relfilename, err := filepath.Rel(cwd, filename); err == nil {
			return relfilename
		}
	}
	return filename
}

// For debugging/informational purposes
func logRuleset(name string, rules []*rule.Rule) {
	if zerolog.GlobalLevel() == zerolog.DebugLevel {
		enabledRules := make([]string, len(rules))
		for i := range rules {
			enabledRules[i] = rules[i].Name
		}
		log.Debug().Strs("rules", enabledRules).Msg(fmt.Sprintf("%s rules", name))
	}
}
