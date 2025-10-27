package maxim

import (
	"fmt"
	"os"
	"sync"

	"github.com/maximhq/maxim-go/apis"
	"github.com/maximhq/maxim-go/logging"
	"github.com/maximhq/maxim-go/prompt"
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

// Maxim is the main client for the Maxim SDK.
// It provides access to various Maxim features like logging.
// The client should be initialized with Init() and cleaned up with Cleanup().
//
// Example usage:
//
//	config := &maxim.MaximSDKConfig{
//		ApiKey: "your-api-key",
//		Debug:  true,
//	}
//	client := maxim.Init(config)
//	defer client.Cleanup()
//
//	// Now you can use the client to access Maxim features
//	logger, err := client.GetLogger(&logging.LoggerConfig{
//		Id: "your-log-repo-id",
//	})
func Init(c *MaximSDKConfig) *Maxim {
	baseUrl := "https://app.getmaxim.ai"
	if c.BaseUrl != nil {
		baseUrl = *c.BaseUrl
	} else if os.Getenv("MAXIM_BASE_URL") != "" {
		baseUrl = os.Getenv("MAXIM_BASE_URL")
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

// GetLogger returns a logger instance for the given configuration.
// It inherits the authentication and debug from Maxim instance.
//
// Example usage:
//
//	config := &maxim.MaximSDKConfig{
//		ApiKey: "your-api-key",
//		Debug:  true,
//	}
//	client := maxim.Init(config)
//	defer client.Cleanup()
//
//	logger, err := client.GetLogger(&logging.LoggerConfig{
//		Id: "your-log-repo-id",
//	})
//	if err != nil {
//		// handle error
//	}
//
// The SDK automatically handles flushing logs when Cleanup() is called.
func (m *Maxim) GetLogger(c *logging.LoggerConfig) (*logging.Logger, error) {
	if c.Id == "" {
		// We will check if its present in the env
		c.Id = os.Getenv("MAXIM_LOG_REPO_ID")
	}
	if c.Id == "" {
		return nil, fmt.Errorf("logger Repo ID is required. Either set it in the config or environment variable MAXIM_LOG_REPO_ID")
	}
	resp := apis.DoesLogRepoExists(m.baseUrl, m.apiKey, c.Id)
	if resp.Error != nil {
		return nil, fmt.Errorf("repo not found %s", resp.Error.Message)
	}
	if _, ok := m.loggers[c.Id]; !ok {
		// Overrides isDebug value from config
		c.IsDebug = m.debug
		m.loggers[c.Id] = logging.NewLogger(m.baseUrl, m.apiKey, c)
	}
	return m.loggers[c.Id], nil
}

// GetPromptVersion fetches a specific version of a prompt.
//
// Parameters:
//   - versionId: The version ID you want to query.
//   - promptId: The prompt ID whose versions you want to query.
//
// Returns:
//   - *apis.PromptVersion: The prompt version if successful, or nil with an error.
//   - error: An error if the request fails.
//
// Example usage:
//
//	client := maxim.Init(&maxim.MaximSDKConfig{
//		ApiKey: "your-api-key",
//	})
//	promptVersion, err := client.GetPromptVersion("version-id", "prompt-id")
//	if err != nil {
//		// handle error
//	}
func (m *Maxim) GetPromptVersion(versionId, promptId string) (*apis.PromptVersion, error) {
	return prompt.GetPromptVersion(m.baseUrl, m.apiKey, versionId, promptId)
}

// Cleanup Maxim SDK state and flushes all logs in all the loggers.
// It should be called when the application is shutting down to ensure all logs are sent to the server.
//
// Example usage:
//
//	config := &maxim.MaximSDKConfig{
//		ApiKey: "your-api-key",
//		Debug:  true,
//	}
//	client := maxim.Init(config)
//	defer client.Cleanup() // This ensures all logs are flushed before the application exits
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
