package maxim

import (
	"fmt"
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
	return &Maxim{
		baseUrl: baseUrl,
		apiKey:  c.ApiKey,
		debug:   c.Debug,
		loggers: map[string]*logging.Logger{},
	}
}

func (m *Maxim) GetLogger(c *logging.LoggerConfig) (*logging.Logger, error) {
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
				logger.Cleanup()
			}(l)
		}
		wg.Wait()
	}
}
