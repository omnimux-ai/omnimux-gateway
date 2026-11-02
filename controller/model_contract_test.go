package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// fork: OmniMux — guards the public capability-contract endpoint.

func contractRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/model_contract/:model", GetModelContract)
	return router
}

func TestModelContractEndpointPublishesADeclaredContract(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/model_contract/seedance-2-5", nil)
	contractRouter().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"model_id":"seedance-2-5"`) {
		t.Errorf("response carries no contract for the requested model: %s", body)
	}
	if !strings.Contains(body, `"version"`) {
		t.Errorf("response carries no payload version, so a client cannot cache safely: %s", body)
	}
	// What the model page renders from must be what a client can read.
	if !strings.Contains(body, `"durations_seconds"`) {
		t.Errorf("response omits the declared duration envelope: %s", body)
	}
}

func TestModelContractEndpointReportsAbsenceForAnUndescribedModel(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/model_contract/no-such-model", nil)
	contractRouter().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestModelContractEndpointResolvesTheDottedSpellingOfAHyphenatedId(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/model_contract/seedance-2.5", nil)
	contractRouter().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for the dotted spelling; body = %s", recorder.Code, recorder.Body.String())
	}
}
