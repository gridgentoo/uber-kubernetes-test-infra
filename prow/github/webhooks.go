/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package github

import (
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

// ValidateWebhook ensures that the provided request conforms to the
// format of a GitHub webhook and the payload can be validated with
// the provided hmac secret. It returns the event type, the event guid,
// the payload of the request, whether the webhook is valid or not,
// and finally the resultant HTTP status code
func ValidateWebhook(w http.ResponseWriter, r *http.Request, tokenGenerator func() []byte) (string, string, []byte, bool, int) {
	defer r.Body.Close()

	// Header checks: It must be a POST with an event type and a signature.
	if r.Method != http.MethodPost {
		responseHTTPError(w, http.StatusMethodNotAllowed, "405 Method not allowed")
		return "", "", nil, false, http.StatusMethodNotAllowed
	}
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		responseHTTPError(w, http.StatusBadRequest, "400 Bad Request: Missing X-GitHub-Event Header")
		return "", "", nil, false, http.StatusBadRequest
	}
	eventGUID := r.Header.Get("X-GitHub-Delivery")
	if eventGUID == "" {
		responseHTTPError(w, http.StatusBadRequest, "400 Bad Request: Missing X-GitHub-Delivery Header")
		return "", "", nil, false, http.StatusBadRequest
	}
	sig := r.Header.Get("X-Hub-Signature")
	if sig == "" {
		responseHTTPError(w, http.StatusForbidden, "403 Forbidden: Missing X-Hub-Signature")
		return "", "", nil, false, http.StatusForbidden
	}
	contentType := r.Header.Get("content-type")
	if contentType != "application/json" {
		responseHTTPError(w, http.StatusBadRequest, "400 Bad Request: Hook only accepts content-type: application/json - please reconfigure this hook on GitHub")
		return "", "", nil, false, http.StatusBadRequest
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		responseHTTPError(w, http.StatusInternalServerError, "500 Internal Server Error: Failed to read request body")
		return "", "", nil, false, http.StatusInternalServerError
	}
	// Validate the payload with our HMAC secret.
	if !ValidatePayload(payload, sig, tokenGenerator) {
		responseHTTPError(w, http.StatusForbidden, "403 Forbidden: Invalid X-Hub-Signature")
		return "", "", nil, false, http.StatusForbidden
	}

	return eventType, eventGUID, payload, true, http.StatusOK
}

func responseHTTPError(w http.ResponseWriter, statusCode int, response string) {
	logrus.WithFields(logrus.Fields{
		"response":    response,
		"status-code": statusCode,
	}).Debug(response)
	http.Error(w, response, statusCode)
}
