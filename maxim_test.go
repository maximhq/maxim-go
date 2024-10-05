package maxim_test

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/maximhq/maxim-go"
	"github.com/maximhq/maxim-go/logging"
)

func uuid() string {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return ""
	}
	// Set version (4) and variant (2)
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

type TestConfig struct {
	BaseUrl string `json:"baseUrl"`
	RepoId  string `json:"repoId"`
	ApiKey  string `json:"apiKey"`
}

func getConfig() *TestConfig {
	// Read testConfig.json file
	file, err := os.Open("testConfig.json")
	if err != nil {
		return nil
	}
	defer file.Close()

	// Decode JSON into TestConfig struct
	testConfigFileData := struct {
		Dev TestConfig `json:"dev"`
	}{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&testConfigFileData)
	if err != nil {
		return nil
	}
	return &testConfigFileData.Dev
}

func TestMaximSDKInit(t *testing.T) {
	tc := getConfig()
	maxim := maxim.Init(&maxim.MaximSDKConfig{
		BaseUrl: &tc.BaseUrl,
		ApiKey:  tc.ApiKey,
		Debug:   true,
	})
	_, err := maxim.GetLogger(&logging.LoggerConfig{Id: tc.RepoId})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMaximSDKTrace(t *testing.T) {
	tc := getConfig()
	mx := maxim.Init(&maxim.MaximSDKConfig{
		BaseUrl: &tc.BaseUrl,
		ApiKey:  tc.ApiKey,
		Debug:   true,
	})
	defer mx.Cleanup()
	logger, err := mx.GetLogger(&logging.LoggerConfig{Id: tc.RepoId})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Cleanup()
	traceId := uuid()
	traceConfig := &logging.TraceConfig{Id: traceId}
	trace := logger.Trace(traceConfig)
	if trace.Id() != traceId {
		t.Fatal("Trace ID mismatch")
	}
	trace.SetOutput("test output")
	trace.End()
	logger.Cleanup()
}

func TestMaximSDKSession(t *testing.T) {
	tc := getConfig()
	mx := maxim.Init(&maxim.MaximSDKConfig{
		BaseUrl: &tc.BaseUrl,
		ApiKey:  tc.ApiKey,
		Debug:   true,
	})
	defer mx.Cleanup()
	logger, err := mx.GetLogger(&logging.LoggerConfig{Id: tc.RepoId})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Cleanup()
	sessionId := uuid()
	sessionConfig := &logging.SessionConfig{Id: sessionId}
	session := logger.Session(sessionConfig)
	traceId := uuid()
	traceConfig := &logging.TraceConfig{Id: traceId}
	trace := session.AddTrace(traceConfig)
	if session.Id() != sessionId {
		t.Fatal("Session ID mismatch")
	}
	if trace.Id() != traceId {
		t.Fatal("Trace ID mismatch")
	}
	trace.SetOutput("test output")
	trace.End()
	session.End()
	logger.Cleanup()
}

func TestMaximSDKSessionChanges(t *testing.T) {
	tc := getConfig()
	maxim := maxim.Init(&maxim.MaximSDKConfig{
		BaseUrl: &tc.BaseUrl,
		ApiKey:  tc.ApiKey,
		Debug:   true,
	})
	defer maxim.Cleanup()
	logger, err := maxim.GetLogger(&logging.LoggerConfig{Id: tc.RepoId})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Cleanup()
	sessionId := uuid()
	sessionConfig := &logging.SessionConfig{Id: sessionId}
	session := logger.Session(sessionConfig)
	traceId := uuid()
	traceConfig := &logging.TraceConfig{Id: traceId}
	trace := session.AddTrace(traceConfig)
	if session.Id() != sessionId {
		t.Fatal("Session ID mismatch")
	}
	if trace.Id() != traceId {
		t.Fatal("Trace ID mismatch")
	}
	trace.SetOutput("test output")
	trace.End()
	session.AddTag("test", "this tag should appear")
	session.End()
	logger.Cleanup()
}

func TestMaximSDKUnendedSession(t *testing.T) {
	tc := getConfig()
	maxim := maxim.Init(&maxim.MaximSDKConfig{
		BaseUrl: &tc.BaseUrl,
		ApiKey:  tc.ApiKey,
		Debug:   true,
	})
	defer maxim.Cleanup()
	logger, err := maxim.GetLogger(&logging.LoggerConfig{Id: tc.RepoId})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Cleanup()
	sessionId := uuid()
	sessionConfig := &logging.SessionConfig{Id: sessionId}
	session := logger.Session(sessionConfig)
	time.Sleep(100 * time.Millisecond)
	session.AddTag("test", "seocond test tag")
	time.Sleep(100 * time.Millisecond)
}

func TestMaximSDKTraceWithGeneration(t *testing.T) {
	tc := getConfig()
	mx := maxim.Init(&maxim.MaximSDKConfig{
		BaseUrl: &tc.BaseUrl,
		ApiKey:  tc.ApiKey,
		Debug:   true,
	})
	defer mx.Cleanup()
	logger, err := mx.GetLogger(&logging.LoggerConfig{Id: tc.RepoId})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Cleanup()
	traceId := uuid()
	trace := logger.Trace(&logging.TraceConfig{Id: traceId})

	time.Sleep(2 * time.Second)
	trace.End()
	logger.AddTagToTrace(trace.Id(), "test", "yes")
	logger.AddEventToTrace(trace.Id(), uuid(), "test event", nil)

	time.Sleep(40 * time.Second)

	generationId := uuid()

	message := logging.CompletionRequest{
		Role:    "user",
		Content: "Hello, how can I help you today ttttt?",
	}
	logger.AddGenerationToTrace(trace.Id(), &logging.GenerationConfig{
		Id:              generationId,
		Name:            maxim.StrPtr("gen1"),
		Provider:        "openai",
		Model:           "gpt-3.5-turbo-16k",
		ModelParameters: map[string]interface{}{"temperature": 3},
		Messages:        []logging.CompletionRequest{message},
	})

	time.Sleep(30 * time.Second)
	logger.AddResultToGeneration(generationId, map[string]interface{}{
		"id":      "10145d10-b2d0-42f6-b69a-9a8311f312b6",
		"object":  "text_completion",
		"created": 1720353381,
		"model":   "gpt-35-turbo",
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"text":          "{\"title\": \"Sending a Greeting in PowerShell\", \"answer\": \"To send a greeting in PowerShell, you can create a cmdlet that accepts a name parameter and writes out a greeting to the user. Here's an example of how you can do it:\\n\\n```powershell\\nusing System.Management.Automation;\\n\\nnamespace SendGreeting\\n{\\n    [Cmdlet(VerbsCommunications.Send, \\\"Greeting\\\")]\\n    public class SendGreetingCommand : Cmdlet\\n    {\\n        [Parameter(Mandatory = true)]\\n        public string Name { get; set; }\\n\\n        protected override void ProcessRecord()\\n        {\\n            WriteObject(\\\"Hello \\\" + Name + \\\"!\\\");\\n        }\\n    }\\n}\\n```\\n\\nYou can then use this cmdlet by calling `Send-Greeting -Name suresh` to send a greeting with the name 'suresh'. The cmdlet will write out 'Hello suresh!' as the output.\", \"source_uuids_scores\": [{\"uuid\": \"c3491cef-0485-3a09-b0cd-41fdf78b160c\", \"score\": 1}] }",
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"completion_tokens": 247,
			"prompt_tokens":     1473,
			"total_tokens":      1720,
		},
	})
}

func TestMaximSDKOutOfOrderMessages(t *testing.T) {
	tc := getConfig()
	mx := maxim.Init(&maxim.MaximSDKConfig{
		BaseUrl: &tc.BaseUrl,
		ApiKey:  tc.ApiKey,
		Debug:   true,
	})
	defer mx.Cleanup()
	logger, err := mx.GetLogger(&logging.LoggerConfig{Id: tc.RepoId})
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Cleanup()

	sessionId := uuid()
	session := logger.Session(&logging.SessionConfig{Id: sessionId})

	traceId := uuid()
	trace := session.AddTrace(&logging.TraceConfig{Id: traceId})

	time.Sleep(2 * time.Second)
	trace.End()

	logger.AddTagToTrace(trace.Id(), "test", "yes")
	logger.AddEventToTrace(trace.Id(), uuid(), "test event", nil)

	time.Sleep(40 * time.Second)

	generationId := uuid()

	message := logging.CompletionRequest{
		Role:    "user",
		Content: "Hello, how can I help you today ttttt?",
	}
	logger.AddGenerationToTrace(trace.Id(), &logging.GenerationConfig{
		Id:              generationId,
		Name:            maxim.StrPtr("gen1"),
		Provider:        "openai",
		Model:           "gpt-3.5-turbo-16k",
		ModelParameters: map[string]interface{}{"temperature": 3},
		Messages:        []logging.CompletionRequest{message},
	})

	time.Sleep(30 * time.Second)

	logger.AddResultToGeneration(generationId, map[string]interface{}{
		"id":      uuid(),
		"object":  "text_completion",
		"created": 1720353381,
		"model":   "gpt-35-turbo",
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"text":          "{\"title\": \"Sending a Greeting in PowerShell\", \"answer\": \"To send a greeting in PowerShell, you can create a cmdlet that accepts a name parameter and writes out a greeting to the user. Here's an example of how you can do it:\\n\\n```powershell\\nusing System.Management.Automation;\\n\\nnamespace SendGreeting\\n{\\n    [Cmdlet(VerbsCommunications.Send, \\\"Greeting\\\")]\\n    public class SendGreetingCommand : Cmdlet\\n    {\\n        [Parameter(Mandatory = true)]\\n        public string Name { get; set; }\\n\\n        protected override void ProcessRecord()\\n        {\\n            WriteObject(\\\"Hello \\\" + Name + \\\"!\\\");\\n        }\\n    }\\n}\\n```\\n\\nYou can then use this cmdlet by calling `Send-Greeting -Name suresh` to send a greeting with the name 'suresh'. The cmdlet will write out 'Hello suresh!' as the output.\", \"source_uuids_scores\": [{\"uuid\": \"c3491cef-0485-3a09-b0cd-41fdf78b160c\", \"score\": 1}] }",
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"completion_tokens": 247,
			"prompt_tokens":     1473,
			"total_tokens":      1720,
		},
	})

	time.Sleep(20 * time.Second)

	span1Id := uuid()
	logger.AddSpanToTrace(trace.Id(), &logging.SpanConfig{Id: span1Id, Name: maxim.StrPtr("Test Span")})

	generation2Id := uuid()
	secondMessage := logging.CompletionRequest{
		Role:    "user",
		Content: "Hello, how can I help you today?",
	}
	logger.AddGenerationToSpan(span1Id, &logging.GenerationConfig{
		Id:              generation2Id,
		Name:            maxim.StrPtr("gen2"),
		Provider:        "openai",
		Model:           "gpt-3.5-turbo-16k",
		ModelParameters: map[string]interface{}{"temperature": 3},
		Messages:        []logging.CompletionRequest{secondMessage},
	})

	time.Sleep(4 * time.Second)

	logger.AddResultToGeneration(generation2Id, map[string]interface{}{
		"id":      "c9395a2d-8fbf-4e96-8ae9-be4820348f46",
		"object":  "text_completion",
		"created": 1720359641,
		"model":   "gpt-35-turbo",
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"text":          "{\"Intent\": \"General Talk\"}",
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{
			"completion_tokens": 7,
			"prompt_tokens":     653,
			"total_tokens":      660,
		},
	})

	time.Sleep(10 * time.Second)

	logger.AddTagToSpan(span1Id, "test", "test-span")
	logger.AddEventToSpan(span1Id, uuid(), "test-event", nil)

	retrievalId := uuid()
	logger.AddRetrievalToSpan(span1Id, &logging.RetrievalConfig{Id: retrievalId, Name: maxim.StrPtr("Test Retrieval")})
	logger.SetRetrievalInput(retrievalId, "asdasdas")
	logger.SetRetrievalOutput(retrievalId, []string{"asdasdas", "asdasdas", "asdasdas"})
	logger.EndRetrieval(retrievalId)

	time.Sleep(2 * time.Second)
	logger.EndSpan(span1Id)
}
