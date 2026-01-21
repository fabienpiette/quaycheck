package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// DockerClient defines the interface for Docker API interactions
type DockerClient interface {
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
}

// Server holds dependencies for the application
type Server struct {
	client DockerClient
}

type PortMapping struct {
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port"`
	Type        string `json:"type"`
	IP          string `json:"ip,omitempty"`
}

type ContainerData struct {
	ID    string        `json:"id"`
	Names []string      `json:"names"`
	Image string        `json:"image"`
	State string        `json:"state"`
	Ports []PortMapping `json:"ports"`
}

type CheckResponse struct {
	Port      int    `json:"port"`
	Available bool   `json:"available"`
	Message   string `json:"message"`
}

type SuggestResponse struct {
	Port    int    `json:"port"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    code,
	})
}

func classifyDockerError(err error) (int, string, string) {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "API version") || strings.Contains(errStr, "client version"):
		return http.StatusBadGateway, "docker_api_version", "Docker API version mismatch. Check socket-proxy compatibility."
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host"):
		return http.StatusServiceUnavailable, "docker_unavailable", "Cannot connect to Docker. Is the daemon running?"
	case strings.Contains(errStr, "permission denied"):
		return http.StatusForbidden, "docker_permission", "Permission denied accessing Docker socket."
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return http.StatusGatewayTimeout, "docker_timeout", "Docker request timed out."
	default:
		return http.StatusInternalServerError, "docker_error", "Docker error: " + errStr
	}
}

func NewDockerClient() (DockerClient, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

func (s *Server) getContainers(ctx context.Context) ([]ContainerData, error) {
	containers, err := s.client.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var result []ContainerData
	for _, c := range containers {
		var ports []PortMapping
		for _, p := range c.Ports {
			ports = append(ports, PortMapping{
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
				IP:          p.IP,
			})
		}

		result = append(result, ContainerData{
			ID:    c.ID,
			Names: c.Names,
			Image: c.Image,
			State: c.State,
			Ports: ports,
		})
	}
	return result, nil
}

func getAllUsedPorts(containers []ContainerData) map[int]bool {
	used := make(map[int]bool)
	for _, c := range containers {
		if c.State == "running" {
			for _, p := range c.Ports {
				used[int(p.PublicPort)] = true
			}
		}
	}
	return used
}

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	containers, err := s.getContainers(r.Context())
	if err != nil {
		status, code, msg := classifyDockerError(err)
		writeError(w, status, code, msg)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containers)
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	portStr := r.URL.Query().Get("port")
	if portStr == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "Missing port parameter")
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_param", "Invalid port parameter")
		return
	}

	containers, err := s.getContainers(r.Context())
	if err != nil {
		status, code, msg := classifyDockerError(err)
		writeError(w, status, code, msg)
		return
	}

	used := getAllUsedPorts(containers)
	available := !used[port]

	msg := "Port is available"
	if !available {
		msg = "Port is currently in use by a Docker container"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CheckResponse{
		Port:      port,
		Available: available,
		Message:   msg,
	})
}

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	if startStr == "" {
		startStr = "8000"
	}
	start, _ := strconv.Atoi(startStr)
	if start < 1024 {
		start = 1024
	}

	containers, err := s.getContainers(r.Context())
	if err != nil {
		status, code, msg := classifyDockerError(err)
		writeError(w, status, code, msg)
		return
	}

	used := getAllUsedPorts(containers)
	suggested := -1

	for i := start; i <= 65535; i++ {
		if !used[i] {
			suggested = i
			break
		}
	}

	msg := fmt.Sprintf("Suggested port: %d", suggested)
	if suggested == -1 {
		msg = "No free ports found in range"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SuggestResponse{
		Port:    suggested,
		Message: msg,
	})
}

// SetupRouter creates and configures the HTTP router
func SetupRouter(server *Server) *http.ServeMux {
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)
	mux.HandleFunc("/api/ports", server.handlePorts)
	mux.HandleFunc("/api/check", server.handleCheck)
	mux.HandleFunc("/api/suggest", server.handleSuggest)
	return mux
}

func main() {
	cli, err := NewDockerClient()
	if err != nil {
		log.Fatalf("Error initializing Docker client: %v", err)
	}

	server := &Server{client: cli}
	mux := SetupRouter(server)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
