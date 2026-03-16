package middlewares

import (
	"github.com/maximhq/maxim-go/logging"
)

// MaximOpenAITransport is an alias for MaximOpenAIAPITransport configured for Azure OpenAI.
// Use NewMaximOpenAITransport for Azure; it delegates to the shared OpenAI middleware.
type MaximOpenAITransport = MaximOpenAIAPITransport

// NewMaximOpenAITransport creates a transport for tracing Azure OpenAI API calls to Maxim.
// It uses the same implementation as OpenAI, with provider set to Azure.
func NewMaximOpenAITransport(logger *logging.Logger) *MaximOpenAITransport {
	return NewMaximOpenAIAPITransportWithProvider(logger, logging.ProviderAzure)
}
