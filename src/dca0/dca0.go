package dca0

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"layeh.com/gopus"
)

type CacheOverflowError struct {
	MaxCacheBytes int
}

func (e *CacheOverflowError) Error() string {
	return "audio too large: the maximum cache limit of " + strconv.Itoa(e.MaxCacheBytes) + " bytes has been exceeded"
}

type Command interface{}

type CommandStop struct{}
type CommandPause struct{}
type CommandResume struct{}
type CommandStartLooping struct{}
type CommandStopLooping struct{}
type CommandSeek float32             // In seconds.
type CommandGetPlaybackTime struct{} // Gets the playback time.
type CommandGetDuration struct{}     // Attempts to get the duration. Only succeeds if the encoder is already done.

type Response interface{}

type ResponsePlaybackTime float32     // Playback time in seconds.
type ResponseDuration float32         // Duration in seconds.
type ResponseDurationUnknown struct{} // Returned if the duration is unknown.

type Dca0Options struct {
	PcmOptions
	// 960 (20ms at 48kHz) is currently the only one supported by discordgo.
	// If it is changed nonetheless, it changes the playback speed (without
	// changing the pitch because of how discord and opus work).
	FrameSize int
	Bitrate   int
	// Maximum number of bytes that may be cached. A 3-minute song usually uses
	// about 2.7MB. If the capacity is full, an error will be sent and the
	// function will exit.
	MaxCacheBytes int
}

func GetDefaultOptions(ffmpegPath string) Dca0Options {
	return Dca0Options{
		PcmOptions: getDefaultPcmOptions(ffmpegPath),
		FrameSize:  960,
		// 64000 is Discord's default.
		Bitrate: 64000,
		// Max cache size of 100MB.
		MaxCacheBytes: 100000000,
	}
}

type Dca0Encoder struct {
	opusEnc *gopus.Encoder
}

func NewEncoder(opts Dca0Options) (*Dca0Encoder, error) {
	opusEnc, err := gopus.NewEncoder(opts.SampleRate, opts.Channels, gopus.Audio)
	if err != nil {
		return nil, err
	}
	return &Dca0Encoder{
		opusEnc: opusEnc,
	}, nil
}

// Sends the individual opus frames as byte arrays through the specified
// channel.
// Input can be either a local file or an http(s) address. It can be of any
// format supported by ffmpeg.
// Caches the entire opus data due to some problems when reading from ffmpeg
// too slowly.
func (e *Dca0Encoder) GetOpusFrames(input string, opts Dca0Options, ch chan<- []byte, errCh chan<- error, cmdCh <-chan Command, respCh chan<- Response) {
	pcm, cmd, err := getPcm(input, opts.PcmOptions)
	if err != nil {
		errCh <- err
		return
	}

	// How many opus frames are played per second.
	framesPerSecond := float32(opts.SampleRate) / float32(opts.FrameSize)
	// Potential maximum samples an audio frame can have.
	maxSamples := opts.FrameSize * opts.Channels
	// One pcm sample equals two bytes.
	maxBytes := maxSamples * 2

	// Size of the opus frame cache.
	cacheSize := 0
	// We're storing all opus frames in this array as a cache.
	opusFrames := make([][]byte, 0, 512)
	// Opus frame read position.
	rp := 0
	// We're sending frames through this before appending them to opusFrames.
	frameCh := make(chan []byte, 8)

	sampleBytes := make([]byte, maxBytes)
	samples := make([]int16, maxSamples)

	// Used by the encoder to tell the main process when it's done encoding.
	encoderDone := make(chan struct{})
	// Used by the main process to tell the encoder to stop.
	encoderStop := make(chan struct{})
	// Launch the encoder.
	// Encode opus data and send it through frameCh.
	go func() {
		var killedFfmpeg bool
	encoderLoop:
		for {
			// Stop encoding if the main process tells us to.
			select {
			case <-encoderStop:
				// Kill ffmpeg using SIGINT.
				cmd.Process.Signal(os.Interrupt)
				pcm.Close()
				killedFfmpeg = true
				// Exit the loop.
				break encoderLoop
			default:
			}

			// Efficiently read the sample bytes outputted by ffmpeg into the
			// int16 sample slice.
			_, err := io.ReadFull(pcm, sampleBytes)
			if err != nil {
				// We also want to stop on ErrUnexpectedEOF because a frame can
				// currently ONLY be 960 * 2 samples large. If a frame were
				// smaller, Discord would just slow the audio down. Since a
				// frame is just 20ms long, we can discard the last one without
				// any issues.
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					break
				} else {
					errCh <- err
				}
			}

			// Read samples from binary, similarly to how the go package binary does
			// it but more efficiently since we're not allocating anything.
			for i := range samples {
				samples[i] = int16(binary.LittleEndian.Uint16(sampleBytes[2*i:]))
			}

			// Encode samples as opus.
			frame, err := e.opusEnc.Encode(samples, opts.FrameSize, maxBytes)
			if err != nil {
				errCh <- err
			}

			if cacheSize > opts.MaxCacheBytes {
				errCh <- &CacheOverflowError{
					MaxCacheBytes: opts.MaxCacheBytes,
				}
				break
			}

			cacheSize += len(frame)
			frameCh <- frame
		}
		// Wait for ffmpeg to close.
		err = cmd.Wait()
		if err != nil {
			// Ffmpeg returns 255 if it was killed using SIGINT. Therefore that
			// wouldn't be an error.
			switch e := err.(type) {
			case *exec.ExitError:
				if e.ExitCode() == 255 {
					if !killedFfmpeg {
						errCh <- err
					}
				}
			default:
				errCh <- err
			}
		}
		// Tell the main process that the encoder is done.
		encoderDone <- struct{}{}
	}()

	encoderRunning := true
	paused := false
	loop := false

loop:
	for {
		select {
		case v := <-frameCh:
			opusFrames = append(opusFrames, v)
		case <-encoderDone:
			encoderRunning = false
		case receivedCmd := <-cmdCh:
			switch v := receivedCmd.(type) {
			case CommandStop:
				if encoderRunning {
					encoderStop <- struct{}{}
				}
				break loop
			case CommandPause:
				paused = true
			case CommandResume:
				paused = false
			case CommandStartLooping:
				loop = true
			case CommandStopLooping:
				loop = false
			case CommandSeek:
				rp = int(float32(v) * framesPerSecond)
			case CommandGetPlaybackTime:
				respCh <- ResponsePlaybackTime(float32(rp) / framesPerSecond)
			case CommandGetDuration:
				if encoderRunning {
					respCh <- ResponseDurationUnknown{}
				} else {
					respCh <- ResponseDuration(float32(len(opusFrames)) / framesPerSecond)
				}
			}
		default:
			time.Sleep(2 * time.Millisecond)
		}

		if !paused && rp < len(opusFrames) {
			if encoderRunning {
				select {
				case ch <- opusFrames[rp]:
					rp++
				default:
				}
			} else {
				ch <- opusFrames[rp]
				rp++
			}
		}

		if !encoderRunning && rp >= len(opusFrames) {
			if loop {
				rp = 0
			} else {
				// We're done sending opus data.
				break
			}
		}
	}

	fmt.Println("Theoretically done calling ffmpeg.")
	fmt.Printf("%s %s\n", cmd.Path, cmd.Args)

	// Wait for the encoder to finish if it's still running.
	if encoderRunning {
		<-encoderDone
		encoderRunning = false
		// TODO: I want to make this unnecessary. I have just noticed that
		// this channel often get stuck so a panic is more helpful than that.
		close(respCh)
	}
}
