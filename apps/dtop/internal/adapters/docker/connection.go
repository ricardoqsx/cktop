package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/moby/moby/client"
	"github.com/ricardoqsx/cktop/apps/dtop/internal/domain"
)

const defaultTimeout = 5 * time.Second

var ErrRemoteUnsupported = domain.ErrRemoteUnsupported

type ConnectionSpec struct {
	Context string
	Host    string
}

type ConnectionInfo struct {
	Name            string
	Endpoint        string
	Transport       string
	Remote          bool
	Secure          bool
	Source          string
	ServerVersion   string
	APIVersion      string
	OperatingSystem string
	NCPU            int
	MemoryTotal     uint64
}

type ResolverOptions struct {
	Spec        ConnectionSpec
	Env         map[string]string
	HomeDir     string
	RuntimeDir  string
	GOOS        string
	AllowRemote bool
}

type endpoint struct {
	name      string
	host      string
	transport string
	remote    bool
	secure    bool
	source    string
}

type Client struct {
	api      *client.Client
	endpoint endpoint
}

func NewClient(ctx context.Context, options ResolverOptions) (*Client, ConnectionInfo, error) {
	resolved, err := ResolveEndpoint(options)
	if err != nil {
		return nil, ConnectionInfo{}, err
	}

	apiClient, err := client.NewClientWithOpts(
		client.WithHost(resolved.host),
		client.WithAPIVersionNegotiation(),
		client.WithTimeout(defaultTimeout),
	)
	if err != nil {
		return nil, ConnectionInfo{}, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	ping, err := apiClient.Ping(pingCtx, client.PingOptions{NegotiateAPIVersion: true})
	if err != nil {
		_ = apiClient.Close()
		return nil, ConnectionInfo{}, fmt.Errorf("connect to Docker Engine at %s: %w", sanitizeEndpoint(resolved.host), err)
	}

	infoCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	engineInfo, err := apiClient.Info(infoCtx, client.InfoOptions{})
	if err != nil {
		_ = apiClient.Close()
		return nil, ConnectionInfo{}, fmt.Errorf("read Docker Engine info at %s: %w", sanitizeEndpoint(resolved.host), err)
	}

	info := resolved.info()
	info.APIVersion = ping.APIVersion
	info.ServerVersion = engineInfo.Info.ServerVersion
	info.OperatingSystem = engineInfo.Info.OperatingSystem
	info.NCPU = engineInfo.Info.NCPU
	if engineInfo.Info.MemTotal > 0 {
		info.MemoryTotal = uint64(engineInfo.Info.MemTotal)
	}

	return &Client{api: apiClient, endpoint: resolved}, info, nil
}

func (c *Client) Close() error {
	if c == nil || c.api == nil {
		return nil
	}

	return c.api.Close()
}

func ResolveEndpoint(options ResolverOptions) (endpoint, error) {
	options = withDefaults(options)

	if options.Spec.Host != "" {
		return validateEndpoint(endpointFromHost(options.Spec.Host, "explicit", options.Spec.Context), options.AllowRemote)
	}

	if host := options.Env["DOCKER_HOST"]; host != "" {
		return validateEndpoint(endpointFromHost(host, "DOCKER_HOST", options.Spec.Context), options.AllowRemote)
	}

	if options.Spec.Context != "" {
		return resolveContext(options, options.Spec.Context, "explicit context")
	}

	if contextName := options.Env["DOCKER_CONTEXT"]; contextName != "" {
		return resolveContext(options, contextName, "DOCKER_CONTEXT")
	}

	if contextName := currentContext(options.HomeDir); contextName != "" {
		resolved, err := resolveContext(options, contextName, "current context")
		if err == nil {
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return endpoint{}, err
		}
	}

	return validateEndpoint(localDefault(options), options.AllowRemote)
}

func (e endpoint) info() ConnectionInfo {
	return ConnectionInfo{
		Name:      fallback(e.name, "default"),
		Endpoint:  sanitizeEndpoint(e.host),
		Transport: e.transport,
		Remote:    e.remote,
		Secure:    e.secure,
		Source:    e.source,
	}
}

func withDefaults(options ResolverOptions) ResolverOptions {
	if options.Env == nil {
		options.Env = readDockerEnv()
	}
	if options.HomeDir == "" {
		options.HomeDir, _ = os.UserHomeDir()
	}
	if options.RuntimeDir == "" {
		options.RuntimeDir = os.Getenv("XDG_RUNTIME_DIR")
	}
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}

	return options
}

func readDockerEnv() map[string]string {
	keys := []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CERT_PATH", "DOCKER_TLS_VERIFY"}
	env := make(map[string]string, len(keys))
	for _, key := range keys {
		env[key] = os.Getenv(key)
	}

	return env
}

func resolveContext(options ResolverOptions, name string, source string) (endpoint, error) {
	host, err := contextHost(options.HomeDir, name)
	if err != nil {
		return endpoint{}, err
	}

	return validateEndpoint(endpointFromHost(host, source, name), options.AllowRemote)
}

func currentContext(homeDir string) string {
	configPath := filepath.Join(homeDir, ".docker", "config.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var config struct {
		CurrentContext string `json:"currentContext"`
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		return ""
	}

	return config.CurrentContext
}

func contextHost(homeDir, name string) (string, error) {
	if name == "default" {
		return "unix:///var/run/docker.sock", nil
	}

	contextsDir := filepath.Join(homeDir, ".docker", "contexts", "meta")
	var found string
	err := filepath.WalkDir(contextsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != "meta.json" {
			return nil
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var meta struct {
			Name      string `json:"Name"`
			Endpoints struct {
				Docker struct {
					Host string `json:"Host"`
				} `json:"docker"`
			} `json:"Endpoints"`
		}
		if err := json.Unmarshal(contents, &meta); err != nil {
			return err
		}
		if meta.Name == name && meta.Endpoints.Docker.Host != "" {
			found = meta.Endpoints.Docker.Host
			return filepath.SkipAll
		}

		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("Docker context %q not found: %w", name, os.ErrNotExist)
	}

	return found, nil
}

func endpointFromHost(host string, source string, name string) endpoint {
	transport := endpointTransport(host)
	secure := transport == "unix" || transport == "npipe" || transport == "ssh" || strings.HasPrefix(host, "https://")
	if transport == "tcp" && strings.HasPrefix(host, "tcp://") {
		secure = false
	}

	return endpoint{
		name:      fallback(name, source),
		host:      host,
		transport: transport,
		remote:    transport == "ssh" || transport == "tcp" || strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://"),
		secure:    secure,
		source:    source,
	}
}

func endpointTransport(host string) string {
	scheme := strings.ToLower(strings.SplitN(host, "://", 2)[0])
	switch scheme {
	case "unix", "npipe", "ssh", "tcp", "http", "https":
		return scheme
	default:
		return "unix"
	}
}

func validateEndpoint(resolved endpoint, allowRemote bool) (endpoint, error) {
	if resolved.remote && !allowRemote {
		return endpoint{}, fmt.Errorf("%w: %s uses %s", ErrRemoteUnsupported, sanitizeEndpoint(resolved.host), resolved.transport)
	}

	return resolved, nil
}

func localDefault(options ResolverOptions) endpoint {
	if options.GOOS == "linux" && options.RuntimeDir != "" {
		rootlessSocket := filepath.Join(options.RuntimeDir, "docker.sock")
		if _, err := os.Stat(rootlessSocket); err == nil {
			return endpointFromHost("unix://"+rootlessSocket, "linux rootless default", "default")
		}
	}

	if options.GOOS == "darwin" && options.HomeDir != "" {
		desktopSocket := filepath.Join(options.HomeDir, ".docker", "run", "docker.sock")
		if _, err := os.Stat(desktopSocket); err == nil {
			return endpointFromHost("unix://"+desktopSocket, "docker desktop default", "desktop-linux")
		}
	}

	return endpointFromHost("unix:///var/run/docker.sock", "platform default", "default")
}

func sanitizeEndpoint(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return raw
	}

	if parsed.User != nil {
		parsed.User = url.User("<user>")
	}
	if parsed.Scheme == "unix" && parsed.Path != "" {
		return "unix://" + parsed.Path
	}

	return parsed.String()
}

func fallback(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
