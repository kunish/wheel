package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
	"github.com/kunish/wheel/apps/worker/internal/relay"
)

const maxRelayRequestBodyBytes = 10 * 1024 * 1024

// relayRequest holds the parsed relay request data.
type relayRequest struct {
	RequestType        string
	IsAnthropicInbound bool
	ContentType        string
	Body               map[string]any
	BodyBytes          []byte
	Model              string
	OriginalModel      string // preserved for logs/metrics after routing rules modify Model
	Stream             bool
	ApiKeyID           int
}

// parseRelayRequest reads the request body, extracts model/stream, and checks access.
// Returns nil if an error response was already written to the client.
func (h *RelayHandler) parseRelayRequest(c *gin.Context) *relayRequest {
	requestType := relay.DetectRequestType(c.Request.URL.Path)
	isAnthropicInbound := requestType == relay.RequestTypeAnthropicMsg
	if requestType == "" {
		apiError(c, http.StatusBadRequest, "invalid_request_error", "Unsupported endpoint", isAnthropicInbound)
		return nil
	}

	bodyBytes, err := readRelayRequestBody(c.Request.Body)
	if err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request_error", err.Error(), isAnthropicInbound)
		return nil
	}

	body, err := parseRelayRequestBody(requestType, c.GetHeader("Content-Type"), bodyBytes)
	if err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request_error", err.Error(), isAnthropicInbound)
		return nil
	}

	model, stream := relay.ExtractModel(body)
	if model == "" && relay.AllowsDefaultMultimodalModel(requestType) {
		model = relay.ExtractMultimodalModel(body, requestType)
	}
	if model == "" {
		apiError(c, 400, "invalid_request_error", "Model is required", isAnthropicInbound)
		return nil
	}

	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)
	if !middleware.CheckModelAccess(sm, model) {
		apiError(c, 403, "invalid_request_error",
			fmt.Sprintf("Model '%s' not allowed for this API key", model),
			isAnthropicInbound,
		)
		return nil
	}

	apiKeyIdRaw, _ := c.Get("apiKeyId")
	apiKeyId, _ := apiKeyIdRaw.(int)

	return &relayRequest{
		RequestType:        requestType,
		IsAnthropicInbound: isAnthropicInbound,
		ContentType:        c.GetHeader("Content-Type"),
		Body:               body,
		BodyBytes:          bodyBytes,
		Model:              model,
		Stream:             stream,
		ApiKeyID:           apiKeyId,
	}
}

func readRelayRequestBody(r io.Reader) ([]byte, error) {
	bodyBytes, err := io.ReadAll(io.LimitReader(r, maxRelayRequestBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("Failed to read request body")
	}
	if len(bodyBytes) > maxRelayRequestBodyBytes {
		return nil, fmt.Errorf("Request body too large")
	}
	return bodyBytes, nil
}

func parseRelayRequestBody(requestType, contentType string, bodyBytes []byte) (map[string]any, error) {
	if relay.RequiresMultipartForm(requestType) {
		return parseMultipartRelayRequestBody(contentType, bodyBytes)
	}
	return parseJSONRelayRequestBody(bodyBytes)
}

func parseJSONRelayRequestBody(bodyBytes []byte) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return nil, fmt.Errorf("Invalid JSON body")
	}
	return body, nil
}

func parseMultipartRelayRequestBody(contentType string, bodyBytes []byte) (map[string]any, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return nil, fmt.Errorf("Unsupported Content-Type")
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, fmt.Errorf("Invalid multipart form data")
	}

	form, err := multipart.NewReader(bytes.NewReader(bodyBytes), boundary).ReadForm(10 << 20)
	if err != nil {
		return nil, fmt.Errorf("Invalid multipart form data")
	}
	defer form.RemoveAll()

	body := make(map[string]any, len(form.Value)+1)
	for key, values := range form.Value {
		switch len(values) {
		case 0:
			continue
		case 1:
			body[key] = values[0]
		default:
			items := make([]any, 0, len(values))
			for _, value := range values {
				items = append(items, value)
			}
			body[key] = items
		}
	}

	files := form.File["file"]
	if len(files) == 0 {
		return nil, fmt.Errorf("File is required")
	}

	file := files[0]
	body["file"] = map[string]any{
		"filename": file.Filename,
		"size":     file.Size,
	}

	return body, nil
}
