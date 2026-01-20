# QuayCheck ⚓

**QuayCheck** is a lightweight, secure, and modern dashboard for managing Docker container ports. It helps you visualize used ports, check availability, and find free ports for your new services.

Built with **Go**, **Pico.css**, and security in mind.

## ✨ Features

- **🛡️ Secure by Design**: Uses a **socket proxy** to isolate the application from the Docker daemon. No direct socket mounting or `privileged` mode required.
- **🔍 Port Scout**: Instantly see which ports are mapped by running containers.
- **✅ Availability Check**: Verify if a specific port (e.g., `8080`) is free.
- **💡 Smart Suggestions**: Get a recommendation for the next available port in a range.
- **🚀 Zero Dependencies**: The frontend is a single HTML file with no build steps, served by a tiny Go binary.

## 🚀 Getting Started

### Prerequisites

- Docker & Docker Compose
- Make (optional, for convenience)

### Quick Start

#### Option A: Clone & Run (For Developers)

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/yourusername/quaycheck.git
    cd quaycheck
    ```

2.  **Start the application:**
    ```bash
    make up
    # OR directly with docker-compose
    docker-compose up -d --build
    ```

#### Option B: Self-Hosted (Docker Compose)

Add the following to your existing `docker-compose.yml` or create a new one:

```yaml
version: '3.8'

services:
  quaycheck:
    image: sighadd/quaycheck:latest
    container_name: quaycheck-app
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - DOCKER_HOST=tcp://socket-proxy:2375
    depends_on:
      - socket-proxy
    networks:
      - dashboard-net

  socket-proxy:
    image: tecnativa/docker-socket-proxy
    container_name: quaycheck-proxy
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - CONTAINERS=1
      - INFO=1
      - VERSION=1
      - POST=0
    networks:
      - dashboard-net

networks:
  dashboard-net:
    driver: bridge
```

3.  **Access the Dashboard:**
    Open [http://localhost:8080](http://localhost:8080) in your browser.

## 🛠️ Configuration

### Environment Variables

The application can be configured via environment variables in `docker-compose.yml`:

| Variable      | Description                                      | Default                     |
| :------------ | :----------------------------------------------- | :-------------------------- |
| `DOCKER_HOST` | URL of the Docker socket or proxy.               | `tcp://socket-proxy:2375`   |
| `PORT`        | The port the Go web server listens on.           | `8080`                      |


## 🏗️ Architecture

QuayCheck prioritizes security by avoiding direct access to `/var/run/docker.sock`.

1.  **Socket Proxy**: A dedicated sidecar container (`tecnativa/docker-socket-proxy`) mounts the Docker socket. It is configured to **only** allow read-only access to container listing APIs (`CONTAINERS=1`, `VERSION=1`).
2.  **App Container**: The Go application connects to the proxy via a private Docker network using TCP. It effectively has "read-only" visibility without the risk of full socket control.

## 🧪 Development

### Running Tests

Run the test suite with coverage reports:

```bash
make test-coverage
```

### Local Build

To build the binary locally (requires Go 1.24+):

```bash
make build
```

## 📄 License

AGPL-3.0 License
