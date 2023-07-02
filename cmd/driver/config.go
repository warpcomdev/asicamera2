package main

import (
	"crypto/tls"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/warpcomdev/asicamera2/internal/driver/backend"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

type Config struct {
	Port                int               `json:"Port" toml:"Port" yaml:"Port"`
	ReadTimeoutSeconds  int               `json:"ReadTimeout" toml:"ReadTimeout" yaml:"ReadTimeout"`
	WriteTimeoutSeconds int               `json:"WriteTimeout" toml:"WriteTimeout" yaml:"WriteTimeout"`
	MaxHeaderBytes      int               `json:"MaxHeaderBytes" toml:"MaxHeaderBytes" yaml:"MaxHeaderBytes"`
	HistoryFolder       string            `json:"HistoryFolder" toml:"HistoryFolder" yaml:"HistoryFolder"`
	LogFolder           string            `json:"LogFolder" toml:"LogFolder" yaml:"LogFolder"`
	MimeTypes           map[string]string `json:"VideoTypes" toml:"VideoTypes" yaml:"VideoTypes"`
	MonitorForMinutes   int               `json:"MonitorForMinutes" toml:"MonitorForMinutes" yaml:"MonitorForMinutes"`
	ExpireAfterDays     int               `json:"ExpireAfterDays" toml:"ExpireAfterDays" yaml:"ExpireAfterDays"`
	ApiUsername         string            `json:"ApiUsername" toml:"ApiUsername" yaml:"ApiUsername"`
	ApiKey              string            `json:"ApiKey" toml:"ApiKey" yaml:"ApiKey"`
	ApiURL              string            `json:"ApiURL" toml:"ApiURL" yaml:"ApiURL"`
	ApiSkipVerify       bool              `json:"ApiSkipVerify" toml:"ApiSkipVerify" yaml:"ApiSkipVerify"`
	ApiRefreshMinutes   int               `json:"ApiRefreshMinutes" toml:"ApiRefreshMinutes" yaml:"ApiRefreshMinutes"`
	ApiTimeoutSeconds   int               `json:"ApiTimeoutSeconds" toml:"ApiTimeoutSeconds" yaml:"ApiTimeoutSeconds"`
	ApiConcurrency      int               `json:"ApiConcurrency" toml:"ApiConcurrency" yaml:"ApiConcurrency"`
	CameraID            string            `json:"CameraID" toml:"CameraID" yaml:"CameraID"`
	Debug               bool              `json:"Debug" toml:"Debug" yaml:"Debug"`
}

func normalizeExtension(ext string) string {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

func (config *Config) Check(configPath string) error {
	if config.Port < 1024 || config.Port > 65535 {
		config.Port = 8080
	}
	if config.ReadTimeoutSeconds < 1 {
		config.ReadTimeoutSeconds = 5
	}
	if config.WriteTimeoutSeconds < 1 {
		config.WriteTimeoutSeconds = 7
	}
	if config.MaxHeaderBytes < 4096 {
		config.MaxHeaderBytes = 1 << 20
	}
	configDir := filepath.Dir(configPath)
	if config.HistoryFolder == "" {
		config.HistoryFolder = filepath.Join(configDir, "history")
	}
	if config.LogFolder == "" {
		config.LogFolder = filepath.Join(configDir, "logs")
	}
	if config.MimeTypes == nil || len(config.MimeTypes) == 0 {
		config.MimeTypes = map[string]string{
			".4gpp":      "video/4gpp",
			".3gpp2":     "video/3gpp2",
			".3gp2":      "video/3gp2",
			".mpg":       "video/mpeg",
			".mp4":       "video/mp4",
			".ogg":       "video/ogg",
			".quicktime": "video/quicktime",
			".webm":      "video/webm",
			".avi":       "video/x-msvideo",
			".jpg":       "image/jpeg",
			".png":       "image/png",
		}
	}
	normalizedTypes := make(map[string]string, len(config.MimeTypes))
	for k, v := range config.MimeTypes {
		normalizedTypes[normalizeExtension(k)] = v
	}
	config.MimeTypes = normalizedTypes
	if config.MonitorForMinutes < 1 {
		config.MonitorForMinutes = 5
	}
	if config.ApiUsername == "" {
		return errors.New("apiUsername config parameter is required")
	}
	if config.ApiKey == "" {
		return errors.New("apiPassword config parameter is required")
	}
	if config.ApiURL == "" {
		return errors.New("apiURL config parameter is required")
	}
	if config.ApiRefreshMinutes < 1 {
		config.ApiRefreshMinutes = 5
	}
	if config.ApiTimeoutSeconds < 1 {
		config.ApiTimeoutSeconds = 10
	}
	if config.ApiConcurrency < 1 {
		config.ApiConcurrency = 3
	}
	if config.ExpireAfterDays < 0 {
		config.ExpireAfterDays = 0
	}
	if config.CameraID == "" {
		return errors.New("cameraID config parameter is required")
	}
	return nil
}

func (c Config) FileTypes() map[string]struct{} {
	buffer := make(map[string]struct{}, len(c.MimeTypes))
	for k := range c.MimeTypes {
		buffer[k] = struct{}{}
	}
	return buffer
}

func (config Config) Server(logger servicelog.Logger) *backend.Server {
	var client backend.Client = &http.Client{
		Timeout: time.Duration(config.ApiTimeoutSeconds) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.ApiSkipVerify,
			},
		},
	}
	if config.Debug {
		client = debugClient{
			logger: logger,
			client: client,
		}
	}
	apiConfig := backend.Config{
		ApiURL:      config.ApiURL,
		Username:    config.ApiUsername,
		Password:    config.ApiKey,
		CameraID:    config.CameraID,
		HTTPTimeout: time.Duration(config.ApiTimeoutSeconds) * time.Second,
		Concurrency: config.ApiConcurrency,
	}
	return backend.New(logger, client, apiConfig)
}
