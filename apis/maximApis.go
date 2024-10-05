package apis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type MaximError struct {
	Message string `json:"message"`
}

// MaximApiResponse represents the structure of the response from the Maxim API.
// It contains an optional Error field which, if present, includes a message describing the error.
type MaximApiResponse struct {
	Error *MaximError `json:"error,omitempty"`
}

func newMaximError(err error) *MaximError {
	return &MaximError{Message: err.Error()}
}

// PushLogs sends logs to the specified repository.
//
// Parameters:
//   - baseUrl: The base URL of the API endpoint.
//   - apiKey: The API key for authentication.
//   - repoId: The ID of the repository where logs will be pushed.
//   - logs: The log data to be pushed.
//
// Returns:
//   - MaximApiResponse: The response from the API, which may contain an error message.
func PushLogs(baseUrl, apiKey, repoId, logs string) MaximApiResponse {
	url := fmt.Sprintf("%s/api/sdk/v3/log?id=%s", baseUrl, repoId)
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, strings.NewReader(logs))
	if err != nil {
		return MaximApiResponse{Error: newMaximError(err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-maxim-api-key", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return MaximApiResponse{Error: newMaximError(err)}
	}
	defer resp.Body.Close()
	var response MaximApiResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return MaximApiResponse{Error: newMaximError(err)}
	}
	return MaximApiResponse{}
}

// DoesLogRepoExists checks if a log repository exists.
//
// Parameters:
//   - baseUrl: The base URL of the API endpoint.
//   - apiKey: The API key for authentication.
//   - repoId: The ID of the repository to check.
//
// Returns:
//   - MaximApiResponse: The response from the API, which may contain an error message.
func DoesLogRepoExists(baseUrl, apiKey, repoId string) MaximApiResponse {
	url := fmt.Sprintf("%s/api/sdk/v3/log-repositories?loggerId=%s", baseUrl, repoId)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return MaximApiResponse{Error: newMaximError(err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-maxim-api-key", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return MaximApiResponse{Error: newMaximError(err)}
	}
	defer resp.Body.Close()
	var response MaximApiResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return MaximApiResponse{Error: newMaximError(err)}
	}
	return MaximApiResponse{}
}
