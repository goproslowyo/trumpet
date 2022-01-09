package main

import (
	"encoding/json"
	"errors"
	"os"
)

type Config struct {
	FfmpegPath                      string   `json:"ffmpeg_path"`
	GoogleServiceAccountCredentials string   `json:"google_service_account_credentials"`
	IgnoreList                      []string `json:"ignore_list"`
	Prefix                          string   `json:"prefix"`
	StreamlinkPath                  string   `json:"streamlink_path"`
	Token                           string   `json:"token"`
	UserAudioPath                   string   `json:"user_audio_path"`
	YtdlPath                        string   `json:"youtube-dl_path"`
}

const configFile = "config.json"

const tokenDefaultString = "insert your discord bot token here"

func ReadConfig(cfg *Config) error {
	configData, err := os.ReadFile(configFile)
	if err != nil {
		return errors.New("unable to read config file: " + err.Error())
	}

	json.Unmarshal(configData, cfg)
	if err != nil {
		return errors.New("unable to decode config file: " + err.Error())
	}
	return nil
}

func WriteDefaultConfig() error {
	data, err := json.MarshalIndent(Config{
		FfmpegPath:                      "ffmpeg",
		GoogleServiceAccountCredentials: "google-translate-api-credentials.json",
		IgnoreList:                      []string{},
		Prefix:                          "!",
		StreamlinkPath:                  "streamlink",
		Token:                           tokenDefaultString,
		UserAudioPath:                   "audio/",
		YtdlPath:                        "youtube-dl",
	}, "", "\t")
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0600)
}
