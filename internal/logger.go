package internal

import (
	"log"
	"os"
)

func NewDebugLogger() *log.Logger {
	return log.New(os.Stdout, "[MaximSDK][Debug]: ", log.Ldate|log.Ltime)
}
