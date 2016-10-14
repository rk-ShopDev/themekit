package kit

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v1"
)

// Configuration is the structure of a configuration for an environment. This will
// get loaded into a theme client to dictate it's actions.
type Configuration struct {
	Password     string        `yaml:"password,omitempty" env:"THEMEKIT_PASSWORD"`
	ThemeID      string        `yaml:"theme_id,omitempty" env:"THEMEKIT_THEME_ID"`
	Domain       string        `yaml:"store" env:"THEMEKIT_DOMAIN"`
	Directory    string        `yaml:"-" env:"THEMEKIT_DIRECTORY"`
	IgnoredFiles []string      `yaml:"ignore_files,omitempty" env:"THEMEKIT_IGNORE_FILES" envSeparator:":"`
	BucketSize   int           `yaml:"bucket_size" env:"THEMEKIT_BUCKET_SIZE"`
	RefillRate   int           `yaml:"refill_rate" env:"THEMEKIT_REFILL_RATE"`
	Concurrency  int           `yaml:"concurrency,omitempty" env:"THEMEKIT_CONCURRENCY"`
	Proxy        string        `yaml:"proxy,omitempty" env:"THEMEKIT_PROXY"`
	Ignores      []string      `yaml:"ignores,omitempty" env:"THEMEKIT_IGNORES" envSeparator:":"`
	Timeout      time.Duration `yaml:"timeout,omitempty" env:"THEMEKIT_TIMEOUT"`
}

const (
	// DefaultBucketSize is the default maximum amount of processes to run at the same time.
	DefaultBucketSize int = 40
	// DefaultRefillRate is the rate in which processes are allowed to spawn at a time.
	DefaultRefillRate int = 2
	// DefaultConcurrency is the default amount of workers that will be spawned for any job.
	DefaultConcurrency int = 2
	// DefaultTimeout is the default timeout to kill any stalled processes.
	DefaultTimeout = 30 * time.Second
)

var (
	defaultConfig     = Configuration{}
	environmentConfig = Configuration{}
	flagConfig        = Configuration{}
)

func init() {
	pwd, _ := os.Getwd()

	defaultConfig = Configuration{
		Directory:   pwd,
		BucketSize:  DefaultBucketSize,
		RefillRate:  DefaultRefillRate,
		Concurrency: DefaultConcurrency,
		Timeout:     DefaultTimeout,
	}

	environmentConfig = Configuration{}
	env.Parse(&environmentConfig)
}

func SetFlagConfig(config Configuration) {
	flagConfig = config
}

// LoadConfiguration will format a Configuration that combines the config from env variables,
// flags and the config file. Then it will validate that config. It will return the
// formatted configuration along with any validation errors. The config precedence
// is flags, environment variables, then the config file.
func NewConfiguration() (Configuration, error) {
	return Configuration{}.compile()
}

func (conf Configuration) compile() (Configuration, error) {
	newConfig := Configuration{}
	mergo.Merge(&newConfig, &flagConfig)
	mergo.Merge(&newConfig, &environmentConfig)
	mergo.Merge(&newConfig, &conf)
	mergo.Merge(&newConfig, &defaultConfig)
	return newConfig, newConfig.Validate()
}

func (conf Configuration) Validate() error {
	errors := []string{}

	if conf.ThemeID == "" {
		errors = append(errors, "missing theme_id")
	} else if !conf.IsLive() {
		if _, err := strconv.ParseInt(conf.ThemeID, 10, 64); err != nil {
			errors = append(errors, "invalid theme_id")
		}
	}

	if len(conf.Domain) == 0 {
		errors = append(errors, "missing domain")
	} else if !strings.HasSuffix(conf.Domain, "myshopify.com") && !strings.HasSuffix(conf.Domain, "myshopify.io") {
		errors = append(errors, "invalid domain, must end in '.myshopify.com'")
	}

	if len(conf.Password) == 0 {
		errors = append(errors, "missing password")
	}

	if len(errors) > 0 {
		return fmt.Errorf("Invalid configuration: %v", strings.Join(errors, ","))
	}
	return nil
}

// AdminURL will return the url to the shopify admin.
func (conf Configuration) AdminURL() string {
	url := fmt.Sprintf("https://%s/admin", conf.Domain)
	if !conf.IsLive() {
		if themeID, err := strconv.ParseInt(conf.ThemeID, 10, 64); err == nil {
			url = fmt.Sprintf("%s/themes/%d", url, themeID)
		}
	}
	return url
}

func (conf Configuration) IsLive() bool {
	return strings.ToLower(strings.TrimSpace(conf.ThemeID)) == "live"
}

// Write will write out a configuration to a writer.
func (conf Configuration) Write(w io.Writer) error {
	bytes, err := yaml.Marshal(conf)
	if err == nil {
		_, err = w.Write(bytes)
	}
	return err
}

// Save will write out the configuration to a file.
func (conf Configuration) Save(location string) error {
	file, err := os.OpenFile(location, os.O_WRONLY|os.O_CREATE, 0644)
	defer file.Close()
	if err == nil {
		err = conf.Write(file)
	}
	return err
}

// AssetPath will return the assets endpoint in the admin section of shopify.
func (conf Configuration) AssetPath() string {
	return fmt.Sprintf("%s/assets.json", conf.AdminURL())
}

// AddHeaders will add api headers to an http.Requests so that it is a valid request.
func (conf Configuration) AddHeaders(req *http.Request) {
	req.Header.Add("X-Shopify-Access-Token", conf.Password)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", fmt.Sprintf("go/themekit (%s; %s)", runtime.GOOS, runtime.GOARCH))
}

// String will return a formatted string with the information about this configuration
func (conf Configuration) String() string {
	return fmt.Sprintf("<token:%s domain:%s bucket:%d refill:%d url:%s>", conf.Password, conf.Domain, conf.BucketSize, conf.RefillRate, conf.AdminURL())
}
