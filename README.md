# Trumpet

This is a Discord Bot. The bot's sole purpose is to announce arrivals and departures in voice chat via TTS. This is [a fork of a lightweight Discord music bot](https://github.com/xypwn/go-musicbot) written in go with minimal dependencies.

## Building from source

- Make sure to have [Go (Golang)](https://golang.org) installed.

- Let Go install all library dependencies: `go mod tidy`.

- Build the program: `go build .`.

## Prerequisites

- Make sure to have [ffmpeg](https://ffmpeg.org/) installed (basically every Linux/BSD distro should have a package for it).

- Make sure to have a **RECENT** version of [youtube-dl](https://yt-dl.org/). If your distro doesn't package a recent version, see the section titled "Obtaining a recent youtube-dl binary".

## Running the binary

Example configuration:

```json
{
	"custom_names": {
		"ExampleUser": "Call Me Something Else"
	},
	"ffmpeg_path": "ffmpeg",
	"google_service_account_credentials": "google-translate-api-credentials.json",
	"ignore_list": [],
	"prefix": "!",
	"token": "insert your discord bot token here",
	"user_audio_path": "audio/",
	"youtube-dl_path": "youtube-dl"
}
```

- Copy `config.json.example config.json` or run the binary once to generate `config.json`: `./trumpet`.

- You can get the TTS api credentials by following instructions [here](https://cloud.google.com/text-to-speech/docs/libraries#setting_up_authentication).

- In `config.json`, find the line that says `"token"`. In that line, change the text that says `"insert your discord bot token here"` to whatever your bot token is (just look it up if you don't know how to get one). Remember to keep the `""` surrounding the token.

- Run the program: `./trumpet`.

## Docker

The code should build and run in docker. The Makefile is opinionated about the volume mounts for configs/audio/etc so check accordingly.

```bash
$ make docker-build
[...snip...]
$ make docker-run
{"level":"info","ts":1658381475.0567544,"caller":"trumpet/config.go:46","msg":"Config file (re)loaded, hash: 99637ef5541bb735d92adad83c646569466c35df71d20190e88b23ab30bf295e\n"}
{"level":"info","ts":1658381475.802256,"caller":"trumpet/main.go:377","msg":"Opened Discord websocket session."}
Bot is now running. Press Ctrl+c to exit.
{"level":"info","ts":1658381475.802329,"caller":"trumpet/main.go:395","msg":"Logged in to Discord as trumpet#7925. Discord UID: 924974173963579392.","username":"trumpet","discriminator":"7925","id":"924974173963579392"}
Logged in as trumpet#7925.
```

### Configuration Options

Useful Note: The program _should_ support config hot-reloading and my minimal testing shows that you can change `config.json` and get a new config loaded without restarting the bot.

There are two main "options" you'll probably adjust often when using trumpet, the ability to "ignore" a user and the ability to have a "custom" vanity name.

The `custom_names` variable contains a key:value mapping of username to preffered "custom" announcement name.

The `ignore_list` variable is simply a list of usernames to ignore so the bot will not announce their join/part events.

## Notes

- youtube-dl might cause some problems with certain Unicode characters if the locale isn't configured correctly (messages like "Adding 0 tracks to queue." may arise). Quick fix: `sudo sh -c "echo 'LC_ALL=\"en_US.UTF-8\"' >> /etc/environment"`.
