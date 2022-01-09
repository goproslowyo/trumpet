// Helper functions for extracting pcm audio from web and local sources.
package dca0

import (
	"errors"
	"io"
	"os/exec"
	"strconv"
)

type PcmOptions struct {
	FfmpegPath string
	Channels   int
	SampleRate int
	Seek       float32 // Seek means where to start in seconds.
	Duration   float32
	// Duration means where to stop (in seconds after the time specified by Seek).
	// If Duration is set to 0, the stream will ignore it and encode all the way
	// to the end of the input.
}

func getDefaultPcmOptions(ffmpegPath string) PcmOptions {
	return PcmOptions{
		FfmpegPath: ffmpegPath,
		Channels:   2,
		// 48000 is the only one used by discord currently.
		SampleRate: 48000,
		Seek:       0,
		Duration:   0,
	}
}

// Input can be either a local file or an http(s) address. It can be of any
// audio format supported by ffmpeg.
// Wait must be called on the returned command to free its resources after
// everything has been read.
func getPcm(input string, opts PcmOptions) (io.ReadCloser, *exec.Cmd, error) {
	if input == "" {
		return nil, nil, errors.New("dca0.getPcm() called with empty input")
	}
	var cmdOpts []string
	cmdOpts = append(cmdOpts, []string{
		"-vn", // No video.
		"-sn", // No subtitle.
		"-dn", // No data encoding.
	}...)
	if opts.Seek != 0.0 {
		cmdOpts = append(cmdOpts,
			"-accurate_seek",
			"-ss", strconv.FormatFloat(float64(opts.Seek), 'f', 5, 32))
	}
	if opts.Duration != 0.0 {
		cmdOpts = append(cmdOpts,
			"-t", strconv.FormatFloat(float64(opts.Duration), 'f', 5, 32))
	}
	cmdOpts = append(cmdOpts, []string{
		"-i", input,
		"-f", "s16le", // Signed int16 samples.
		"-ar", strconv.Itoa(opts.SampleRate),
		"-ac", strconv.Itoa(opts.Channels), // Number of audio channels.
		"pipe:1", // Output to stdout.
	}...)
	cmd := exec.Command(opts.FfmpegPath, cmdOpts...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdout, cmd, nil
}
