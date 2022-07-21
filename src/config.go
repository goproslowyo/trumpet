package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Config struct {
	CustomNames                     map[string]string `json:"custom_names"`
	FfmpegPath                      string            `json:"ffmpeg_path"`
	AnnouncementPath                string            `json:"announcement_path"`
	GoogleServiceAccountCredentials string            `json:"google_service_account_credentials"`
	IgnoreList                      []string          `json:"ignore_list"`
	Prefix                          string            `json:"prefix"`
	Token                           string            `json:"token"`
	UserAudioPath                   string            `json:"user_audio_path"`
	YtdlPath                        string            `json:"youtube-dl_path"`
	ConfigHash                      string
}

const configFile = "config.json"

const tokenDefaultString = "insert your discord bot token here"

func (c *Config) hashConfig(config []byte) string {
	hash := fmt.Sprintf("%x", sha256.Sum256(config))
	return hash
}

func ReadConfig(cfg *Config) error {
	configData, err := os.ReadFile(configFile)
	if err != nil {
		return errors.New("unable to read config file: " + err.Error())
	}
	newHash := cfg.hashConfig(configData)

	if cfg.ConfigHash != newHash || cfg.ConfigHash == "" {
		json.Unmarshal(configData, cfg)
		if err != nil {
			return errors.New("unable to decode config file: " + err.Error())
		}
		cfg.ConfigHash = newHash
		logger.Sugar().Infof("Config file (re)loaded, hash: %s\n", cfg.ConfigHash)
	}
	return nil
}

func WriteDefaultConfig() error {
	data, err := json.MarshalIndent(Config{
		CustomNames:                     map[string]string{},
		FfmpegPath:                      "ffmpeg",
		AnnouncementPath:                "announcements",
		GoogleServiceAccountCredentials: "google-translate-api-credentials.json",
		IgnoreList:                      []string{},
		Prefix:                          "!",
		Token:                           tokenDefaultString,
		UserAudioPath:                   "audio/",
		YtdlPath:                        "youtube-dl",
	}, "", "\t")
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0600)
}
