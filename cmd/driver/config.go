package main

import (
	"crypto/tls"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/warpcomdev/asicamera2/internal/driver/backend"
	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

type Config struct {
	Port                int               `json:"port" toml:"port" yaml:"port"`
	ReadTimeoutSeconds  int               `json:"readTimeout" toml:"readTimeout" yaml:"readTimeout"`
	WriteTimeoutSeconds int               `json:"writeTimeout" toml:"writeTimeout" yaml:"writeTimeout"`
	MaxHeaderBytes      int               `json:"maxHeaderBytes" toml:"maxHeaderBytes" yaml:"maxHeaderBytes"`
	HistoryFolder       string            `json:"historyFolder" toml:"historyFolder" yaml:"historyFolder"`
	MimeTypes           map[string]string `json:"videoTypes" toml:"videoTypes" yaml:"videoTypes"`
	MonitorForMinutes   int               `json:"monitorForMinutes" toml:"monitorForMinutes" yaml:"monitorForMinutes"`
	ApiUsername         string            `json:"apiUsername" toml:"apiUsername" yaml:"apiUsername"`
	ApiKey              string            `json:"apiKey" toml:"apiKey" yaml:"apiKey"`
	ApiURL              string            `json:"apiURL" toml:"apiURL" yaml:"apiURL"`
	ApiSkipVerify       bool              `json:"apiSkipVerify" toml:"apiSkipVerify" yaml:"apiSkipVerify"`
	ApiRefreshMinutes   int               `json:"apiRefreshMinutes" toml:"apiRefreshMinutes" yaml:"apiRefreshMinutes"`
	ApiTimeoutSeconds   int               `json:"apiTimeoutSeconds" toml:"apiTimeoutSeconds" yaml:"apiTimeoutSeconds"`
	ApiConcurrency      int               `json:"apiConcurrency" toml:"apiConcurrency" yaml:"apiConcurrency"`
	CameraID            string            `json:"cameraID" toml:"cameraID" yaml:"cameraID"`
	Debug               bool              `json:"debug" toml:"debug" yaml:"debug"`
}

func normalizeExtension(ext string) string {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

func (config *Config) Check() error {
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
	if config.HistoryFolder == "" {
		return errors.New("historyFolder config parameter is required")
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
