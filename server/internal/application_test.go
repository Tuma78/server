package application

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envPort  string
		expected string
	}{
		{
			name:     "With PORT env set",
			envPort:  "3000",
			expected: "3000",
		},
		{
			name:     "Without PORT env",
			envPort:  "",
			expected: "8080",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envPort != "" {
				os.Setenv("PORT", tc.envPort)
				defer os.Unsetenv("PORT")
			} else {
				os.Unsetenv("PORT")
			}

			config := ConfigFromEnv()
			if config.Addr != tc.expected {
				t.Errorf("Expected port %s, got %s", tc.expected, config.Addr)
			}
		})
	}
}

func TestNew(t *testing.T) {
	app := New()
	if app == nil {
		t.Error("Expected non-nil application")
	}
	if app.config == nil {
		t.Error("Expected non-nil config")
	}
}

func TestCalcHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           Request
		expectedStatus int
		expectedBody   Response
	}{
		{
			name:           "Valid expression",
			method:         http.MethodPost,
			body:           Request{Expression: "2 + 2"},
			expectedStatus: http.StatusOK,
			expectedBody:   Response{Result: "4.000000"},
		},
		{
			name:           "Invalid method",
			method:         http.MethodGet,
			body:           Request{},
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Invalid expression characters",
			method:         http.MethodPost,
			body:           Request{Expression: "2 + 2 = ?"},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   Response{Error: "Expression is not valid"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(tc.method, "/api/v1/calculate", bytes.NewBuffer(bodyBytes))
			w := httptest.NewRecorder()

			CalcHandler(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedStatus == http.StatusOK || tc.expectedStatus == http.StatusUnprocessableEntity {
				var response Response
				err := json.NewDecoder(w.Body).Decode(&response)
				if err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}

				if tc.expectedBody.Result != "" && response.Result != tc.expectedBody.Result {
					t.Errorf("Expected result %s, got %s", tc.expectedBody.Result, response.Result)
				}
				if tc.expectedBody.Error != "" && response.Error != tc.expectedBody.Error {
					t.Errorf("Expected error %s, got %s", tc.expectedBody.Error, response.Error)
				}
			}
		})
	}
}

func TestIsValidExpression(t *testing.T) {
	tests := []struct {
		expression string
		expected   bool
	}{
		{"2 + 2", true},
		{"2.5 * 3.7", true},
		{"(1 + 2) * 3", true},
		{"2 + 2 = 4", false},
		{"hello", false},
		{"2 ^ 2", false},
	}

	for _, tc := range tests {
		result := isValidExpression(tc.expression)
		if result != tc.expected {
			t.Errorf("For expression %q: expected %v, got %v", tc.expression, tc.expected, result)
		}
	}
}

func TestIsValidChar(t *testing.T) {
	validChars := []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'+', '-', '*', '/', '(', ')', '.', ' '}
	invalidChars := []rune{'=', '^', 'a', '$', '&'}

	for _, char := range validChars {
		if !isValidChar(char) {
			t.Errorf("Character %q should be valid", char)
		}
	}

	for _, char := range invalidChars {
		if isValidChar(char) {
			t.Errorf("Character %q should be invalid", char)
		}
	}
}