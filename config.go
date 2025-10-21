package main

import (
	"bytes"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

// Config holds the application configuration.
type Config struct {
	PhotoUploadDir string `toml:"photo_upload_dir"`
	DataDir        string `toml:"data_dir"`
}

// AppConfig is the global configuration instance.
var AppConfig Config

// LoadConfig loads the configuration from a file, creating a default if it doesn't exist.
func LoadConfig() {
	// Set default configuration
	AppConfig = Config{
		PhotoUploadDir: "/data/tmunot",
		DataDir:        "data",
	}

	configFilePath := "config.toml"
	_, err := os.Stat(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("%s not found, creating with default values.", configFilePath)
			buf := new(bytes.Buffer)
			if err := toml.NewEncoder(buf).Encode(AppConfig); err != nil {
				log.Fatalf("Error encoding default config: %v", err)
			}
			if err := os.WriteFile(configFilePath, buf.Bytes(), 0644); err != nil {
				log.Fatalf("Error writing default config file: %v", err)
			}
		} else {
			log.Printf("Warning: could not stat %s, using default configuration: %v", configFilePath, err)
		}
		return // Use defaults
	}

	if _, err := toml.DecodeFile(configFilePath, &AppConfig); err != nil {
		log.Fatalf("Error decoding %s: %v. Please check its format.", configFilePath, err)
	}
	log.Printf("Configuration loaded from %s", configFilePath)
}