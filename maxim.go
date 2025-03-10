package maxim

import (
	"fmt"
	"os"
	"sync"

	"github.com/maximhq/maxim-go/apis"
	"github.com/maximhq/maxim-go/logging"
)

type MaximSDKConfig struct {
	BaseUrl *string
	ApiKey  string
	Debug   bool
}

type Maxim struct {
	baseUrl string
	apiKey  string
	debug   bool
	loggers map[string]*logging.Logger
}

func Init(c *MaximSDKConfig) *Maxim {
	baseUrl := "https://app.getmaxim.ai"
	if c.BaseUrl != nil {
		baseUrl = *c.BaseUrl
	}
	apiKey := c.ApiKey
	if c.ApiKey == "" {
		apiKey = os.Getenv("MAXIM_API_KEY")
	}
	return &Maxim{
		baseUrl: baseUrl,
		apiKey:  apiKey,
		debug:   c.Debug,
		loggers: map[string]*logging.Logger{},
	}
}

func (m *Maxim) GetLogger(c *logging.LoggerConfig) (*logging.Logger, error) {
	if c.Id == "" {
		// We will check if its present in the env
		c.Id = os.Getenv("MAXIM_LOG_REPO_ID")
	}
	if c.Id == "" {
		return nil, fmt.Errorf("Logger Repo ID is required. Either set it in the config or environment variable MAXIM_LOG_REPO_ID")
	}
	resp := apis.DoesLogRepoExists(m.baseUrl, m.apiKey, c.Id)
	if resp.Error != nil {
		return nil, fmt.Errorf("Repo not found %s", resp.Error.Message)
	}
	if _, ok := m.loggers[c.Id]; !ok {
		// Overrides isDebug value from config
		c.IsDebug = m.debug
		m.loggers[c.Id] = logging.NewLogger(m.baseUrl, m.apiKey, c)
	}
	return m.loggers[c.Id], nil
}

func (m *Maxim) Cleanup() {
	if m.loggers != nil {
		var wg sync.WaitGroup
		for _, l := range m.loggers {
			wg.Add(1)
			go func(logger *logging.Logger) {
				defer wg.Done()
				logger.Flush()
			}(l)
		}
		wg.Wait()
	}
}
