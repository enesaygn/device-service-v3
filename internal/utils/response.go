// internal/utils/response.go
package utils

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// APIResponse represents standard API response structure
type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// APIError represents error information
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SuccessResponse sends a successful response
func SuccessResponse(c *gin.Context, statusCode int, message string, data interface{}) {
	response := APIResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
		RequestID: getRequestID(c),
	}

	c.JSON(statusCode, response)
}

// ErrorResponse sends an error response
func ErrorResponse(c *gin.Context, statusCode int, message string, err error) {
	apiError := &APIError{
		Code:    getErrorCode(statusCode),
		Message: message,
	}

	if err != nil {
		apiError.Details = err.Error()
	}

	response := APIResponse{
		Success:   false,
		Message:   message,
		Error:     apiError,
		Timestamp: time.Now(),
		RequestID: getRequestID(c),
	}

	c.JSON(statusCode, response)
}

// ValidationErrorResponse sends validation error response
func ValidationErrorResponse(c *gin.Context, errors map[string]string) {
	apiError := &APIError{
		Code:    "VALIDATION_ERROR",
		Message: "Request validation failed",
	}

	response := APIResponse{
		Success:   false,
		Message:   "Validation failed",
		Error:     apiError,
		Data:      gin.H{"validation_errors": errors},
		Timestamp: time.Now(),
		RequestID: getRequestID(c),
	}

	c.JSON(http.StatusBadRequest, response)
}

// getRequestID extracts request ID from context
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		return requestID.(string)
	}
	return ""
}

// getErrorCode returns error code based on HTTP status
func getErrorCode(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMIT_EXCEEDED"
	case http.StatusInternalServerError:
		return "INTERNAL_SERVER_ERROR"
	case http.StatusServiceUnavailable:
		return "SERVICE_UNAVAILABLE"
	default:
		return "UNKNOWN_ERROR"
	}
}
