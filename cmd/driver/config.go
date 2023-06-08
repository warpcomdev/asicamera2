package main

type Config struct {
	Port           int      `json:"port"`
	ReadTimeout    int      `json:"readTimeout"`
	WriteTimeout   int      `json:"writeTimeout"`
	MaxHeaderBytes int      `json:"maxHeaderBytes"`
	HistoryFolder  string   `json:"historyFolder"`
	VideoTypes     []string `json:"videoTypes"`
	PictureTypes   []string `json:"pictureTypes"`
	MonitorFor     int      `json:"monitorFor"` // in minutes
	Debug          bool     `json:"debug"`
}

func newConfig() *Config {
	return &Config{
		Port:           8080,
		ReadTimeout:    5,
		WriteTimeout:   7,
		MaxHeaderBytes: 1 << 20,
		HistoryFolder:  "C:\\XboxGames\\history",
		PictureTypes:   []string{".jpg", ".jpeg", ".png", ".gif", ".bmp"},
		VideoTypes:     []string{".avi", ".vid", ".mpg", ".mp4", ".av1"},
		Debug:          true,
		MonitorFor:     5,
	}
}

func (c Config) FileTypes() []string {
	buffer := append(make([]string, 0, len(c.PictureTypes)+len(c.VideoTypes)), c.PictureTypes...)
	return append(buffer, c.VideoTypes...)
}
