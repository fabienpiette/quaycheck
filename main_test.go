package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types"
)

// MockDockerClient is a mock implementation of DockerClient
type MockDockerClient struct {
	Containers []types.Container
	Err        error
}

func (m *MockDockerClient) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Containers, nil
}

func TestGetContainers(t *testing.T) {
	mockContainers := []types.Container{
		{
			ID:    "123",
			Names: []string{"/test1"},
			Image: "image1",
			State: "running",
			Ports: []types.Port{
				{PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
			},
		},
	}

	mockClient := &MockDockerClient{Containers: mockContainers}
	server := &Server{client: mockClient}

	containers, err := server.getContainers(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(containers))
	}

	if containers[0].ID != "123" {
		t.Errorf("Expected ID 123, got %s", containers[0].ID)
	}
}

func TestGetAllUsedPorts(t *testing.T) {
	containers := []ContainerData{
		{
			State: "running",
			Ports: []PortMapping{
				{PublicPort: 8080},
				{PublicPort: 9090},
			},
		},
		{
			State: "exited",
			Ports: []PortMapping{
				{PublicPort: 3000},
			},
		},
	}

	used := getAllUsedPorts(containers)

	if !used[8080] {
		t.Error("Expected 8080 to be used")
	}
	if !used[9090] {
		t.Error("Expected 9090 to be used")
	}
	if used[3000] {
		t.Error("Expected 3000 to NOT be used (container exited)")
	}
}

func TestHandlePorts(t *testing.T) {
	mockContainers := []types.Container{
		{
			ID:    "123",
			Names: []string{"/test1"},
			Ports: []types.Port{{PublicPort: 8080}},
		},
	}
	mockClient := &MockDockerClient{Containers: mockContainers}
	server := &Server{client: mockClient}

	req := httptest.NewRequest("GET", "/api/ports", nil)
	w := httptest.NewRecorder()

	server.handlePorts(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result []ContainerData
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) != 1 {
		t.Errorf("Expected 1 container in response")
	}
}

func TestHandleCheck(t *testing.T) {
	mockContainers := []types.Container{
		{
			State: "running",
			Ports: []types.Port{{PublicPort: 8080}},
		},
	}
	mockClient := &MockDockerClient{Containers: mockContainers}
	server := &Server{client: mockClient}

	tests := []struct {
		port      string
		available bool
		status    int
	}{
		{"8080", false, http.StatusOK},
		{"9000", true, http.StatusOK},
		{"invalid", false, http.StatusBadRequest},
		{"", false, http.StatusBadRequest},
	}

	for _, tt := range tests {
		url := "/api/check"
		if tt.port != "" {
			url += "?port=" + tt.port
		}
		req := httptest.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()

		server.handleCheck(w, req)

		resp := w.Result()
		if resp.StatusCode != tt.status {
			t.Errorf("Port %s: Expected status %d, got %d", tt.port, tt.status, resp.StatusCode)
		}

		if resp.StatusCode == http.StatusOK {
			var result CheckResponse
			json.NewDecoder(resp.Body).Decode(&result)
			if result.Available != tt.available {
				t.Errorf("Port %s: Expected available=%v, got %v", tt.port, tt.available, result.Available)
			}
		}
	}
}

func TestHandleSuggest(t *testing.T) {
	mockContainers := []types.Container{
		{
			State: "running",
			Ports: []types.Port{{PublicPort: 8000}, {PublicPort: 8001}},
		},
	}
	mockClient := &MockDockerClient{Containers: mockContainers}
	server := &Server{client: mockClient}

	tests := []struct {
		startParam    string
		expectedPort  int
	}{
		{"8000", 8002}, // 8000, 8001 used
		{"9000", 9000}, // 9000 free
		{"10", 1024},   // Too low, defaults to 1024 (assuming 1024 is free)
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/api/suggest?start="+tt.startParam, nil)
		w := httptest.NewRecorder()

		server.handleSuggest(w, req)

		resp := w.Result()
		var result SuggestResponse
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Port != tt.expectedPort {
			t.Errorf("Start %s: Expected port %d, got %d", tt.startParam, tt.expectedPort, result.Port)
		}
	}
}

func TestHandleErrors(t *testing.T) {
	mockClient := &MockDockerClient{Err: errors.New("docker down")}
	server := &Server{client: mockClient}

	// Test handlePorts error
	req := httptest.NewRequest("GET", "/api/ports", nil)
	w := httptest.NewRecorder()
	server.handlePorts(w, req)
	if w.Result().StatusCode != http.StatusInternalServerError {
		t.Error("Expected 500 on handlePorts error")
	}

	// Test handleCheck error
	req = httptest.NewRequest("GET", "/api/check?port=8080", nil)
	w = httptest.NewRecorder()
	server.handleCheck(w, req)
	if w.Result().StatusCode != http.StatusInternalServerError {
		t.Error("Expected 500 on handleCheck error")
	}

	// Test handleSuggest error
	req = httptest.NewRequest("GET", "/api/suggest", nil)
	w = httptest.NewRecorder()
	server.handleSuggest(w, req)
	if w.Result().StatusCode != http.StatusInternalServerError {
		t.Error("Expected 500 on handleSuggest error")
	}
}

func TestNewDockerClient(t *testing.T) {
	_, _ = NewDockerClient()
}

func TestPortMappingStructure(t *testing.T) {
	pm := PortMapping{PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: "0.0.0.0"}
	if pm.PublicPort != 8080 {
		t.Error("Struct field access failed")
	}
}

func TestSetupRouter(t *testing.T) {
	server := &Server{client: &MockDockerClient{}}
	mux := SetupRouter(server)
	if mux == nil {
		t.Error("Expected mux to be not nil")
	}
	
	req := httptest.NewRequest("GET", "/api/ports", nil)
	_, pattern := mux.Handler(req)
	// In Go 1.22+ mux.Handler returns pattern, but here it returns handler and pattern string.
	// Since we can't easily check internal mux state, we just verify it handles the request.
	if pattern == "" {
		// Fallback check if pattern matching behaves differently
	}
	
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Error("Expected router to wire handlePorts correctly")
	}
}