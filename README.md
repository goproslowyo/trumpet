# Trumpet

This is a Discord Bot. The bot's sole purpose is to announce arrivals and departures in voice chat via TTS. This is [a fork of a lightweight Discord music bot](https://github.com/xypwn/go-musicbot) written in go with minimal dependencies.

## Building from source

- Make sure to have [Go (Golang)](https://golang.org) installed.

- Let Go install all library dependencies: `go mod tidy`.

- Build the program: `go build .`.

## Running the binary

- Run the binary once to generate `config.json`: `./dcbot`.

- Make sure to have [ffmpeg](https://ffmpeg.org/) installed (basically every Linux/BSD distro should have a package for it).

- Make sure to have a **RECENT** version of [youtube-dl](https://yt-dl.org/). If your distro doesn't package a recent version, see the section titled "Obtaining a recent youtube-dl binary".

- In `config.json`, find the line that says `"token"`. In that line, change the text that says `"insert your discord bot token here"` to whatever your bot token is (just look it up if you don't know how to get one). Remember to keep the `""` sorrounding the token.

## Notes

- youtube-dl might cause some problems with certain Unicode characters if the locale isn't configured correctly (messages like "Adding 0 tracks to queue." may arise). Quick fix: `sudo sh -c "echo 'LC_ALL=\"en_US.UTF-8\"' >> /etc/environment"`.

## Setup

### Obtaining a recent youtube-dl binary

Some distributions like Debian don't ship a very recent version of youtube-dl, but there is a simple solution for running this bot nonetheless.

- Locally download the latest youtube-dl version **into the cloned repo**: `wget https://yt-dl.org/downloads/latest/youtube-dl`.

- Make youtube-dl executable `chmod +x ./youtube-dl`.

- In `config.json`, look for the line that says `"youtube-dl_path"`. In that line, change the value that says `"youtube-dl"` to `"./youtube-dl"`.

- Save and you're done. The bot should now look for youtube-dl locally instead of system-wide.
