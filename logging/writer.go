package logging

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/maximhq/maxim-go/apis"
	"github.com/maximhq/maxim-go/internal"
	"github.com/maximhq/maxim-go/utils"
)

type writerConfig struct {
	BaseUrl              string
	ApiKey               string
	RepoId               string
	AutoFlush            bool
	FlushIntervalSeconds int
	IsDebug              bool
}

type writer struct {
	config  *writerConfig
	queue   *utils.Queue[*CommitLog]
	mutex   *utils.Mutex
	ticker  *time.Ticker
	isDebug bool
	logsDir string
	logger  *log.Logger
}

// NewWriter creates a new Writer instance
func newWriter(c *writerConfig) *writer {
	w := &writer{
		ticker:  time.NewTicker(time.Duration(c.FlushIntervalSeconds) * time.Second),
		config:  c,
		logsDir: fmt.Sprintf("%smaxim-sdk/%s/maxim-logs", os.TempDir(), c.RepoId),
		queue:   utils.NewQueue[*CommitLog](),
		mutex:   utils.NewMutex(),
		isDebug: c.IsDebug,
	}
	if w.isDebug {
		w.logger = internal.NewDebugLogger()
	}
	w.init()
	return w
}

func (w *writer) init() {
	go func() {
		for {
			select {
			case <-w.ticker.C:
				w.flush()
			}
		}
	}()
}

func (w *writer) writeToFile(logs []*CommitLog) error {
	if _, err := os.Stat(w.logsDir); os.IsNotExist(err) {
		err := os.MkdirAll(w.logsDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		}
	}
	content := ""
	for _, log := range logs {
		content += log.Serialize() + "\n"
	}
	fileName := fmt.Sprintf("%s/%s.log", w.logsDir, time.Now().Format("2006-01-02"))
	err := os.WriteFile(fileName, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write logs to file: %w", err)
	}
	return nil
}

func (w *writer) flushLogFiles() error {
	if _, err := os.Stat(w.logsDir); os.IsNotExist(err) {
		return nil
	}
	files, err := os.ReadDir(w.logsDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := fmt.Sprintf("%s/%s", w.logsDir, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		resp := apis.PushLogs(w.config.BaseUrl, w.config.ApiKey, w.config.RepoId, string(content))
		if resp.Error != nil {
			continue
		}
		os.Remove(filePath)
	}
	return nil
}

func (w *writer) flushLogs(logs []*CommitLog) error {
	_, err := os.Stat(w.logsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check logs directory: %w", err)
	}
	err = w.flushLogFiles()
	if err != nil {
		log.Println("[MaximSDK][Error]: error while flushing log files %w", err.Error())
	}
	if w.logger != nil {
		w.logger.Println("flushing logs to server")
		w.logger.Println("[===========[LOG START]==============]")
	}
	content := ""
	for _, log := range logs {
		if w.logger != nil {
			w.logger.Println(log.Serialize())
		}
		content += log.Serialize() + "\n"
	}
	if w.logger != nil {
		w.logger.Println("[===========[LOG END]==============]")
	}
	resp := apis.PushLogs(w.config.BaseUrl, w.config.ApiKey, w.config.RepoId, content)
	if resp.Error != nil {
		log.Println("[MaximSDK][Error]: failed to push logs to server: %w", resp.Error.Message)
		err := w.writeToFile(logs)
		if err != nil {
			log.Println("[MaximSDK][Error]: failed to backup logs to file: %w", err)
		}
		return fmt.Errorf("%s", resp.Error.Message)
	}
	if w.logger != nil {
		w.logger.Println("logs pushed to server successfully")
	}
	return nil
}

func (w *writer) flush() {
	err := w.mutex.Acquire()
	if err != nil {
		return
	}
	defer w.mutex.Release()
	if w.logger != nil {
		w.logger.Println("flushing logs")
	}
	logs := w.queue.DequeueAll()
	if len(logs) == 0 {
		if w.logger != nil {
			w.logger.Println("no logs to flush")
		}
		return
	}
	err = w.flushLogs(logs)
	if err != nil {
		log.Println("MaximSDK][Error]: error while flushing logs: ", err.Error())
		return
	}
	if w.logger != nil {
		w.logger.Println("logs flushed successfully")
	}
}

func (w *writer) commit(cl *CommitLog) {
	if w.logger != nil {
		w.logger.Println("Committing log: ", cl.Serialize())
	}
	w.queue.Enqueue(cl)
}

func (w *writer) cleanup() {
	w.flush()
	w.ticker.Stop()
}
