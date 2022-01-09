package ytdl

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
)

type Extractor struct {
	DefaultSearch string
	YtdlPath      string
}

func NewExtractor(ytdlPath string) *Extractor {
	return &Extractor{
		DefaultSearch: "ytsearch",
		YtdlPath:      ytdlPath,
	}
}

var (
	errInvalidMetadata = errors.New("invalid metadata received from youtube-dl")
)

type Metadata map[string]interface{}

/*// `input` can be a URL or a search query.
// Returns a slice with size 1 if the input is a single media file. Returns a
// larger slice if the input is a playlist.
// Progress sends a struct{} every time a single item has been added.
func (e *Extractor) GetMetadata(input string, progress chan<- struct{}) ([]Metadata, error) {
	cmd := exec.Command(e.YtdlPath, "--default-search", e.DefaultSearch, "-j", input)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return nil, newError(input, err)
	}

	defer close(progress)

	var m sync.Mutex
	var done bool
	var ret []Metadata
	var ytdlErr error
	var killedYtdl bool
	go func() {
		killYtdl := func(err error) {
			m.Lock()
			ytdlErr = err
			cmd.Process.Signal(os.Interrupt)
			killedYtdl = true
			m.Unlock()
		}

		dec := json.NewDecoder(&out)
		m.Lock()
		d := done
		m.Unlock()
		for !d && dec.More() {
			var meta interface{}
			if err := dec.Decode(&meta); err != nil {
				killYtdl(newError(input, err))
			}
			progress <- struct{}{}

			if m, ok := meta.(map[string]interface{}); ok {
				ret = append(ret, m)
			} else {
				killYtdl(newError(input, errInvalidMetadata))
			}
		}
	}()

	err := cmd.Wait()
	m.Lock()
	done = true
	m.Unlock()
	if ytdlErr != nil {
		return nil, ytdlErr
	}
	if err != nil && !killedYtdl {
		return nil, newError(input, err)
	}
	return ret, nil
}*/

// `input` can be a URL or a search query.
// Returns a slice with size 1 if the input is a single media file. Returns a
// larger slice if the input is a playlist.
func (e *Extractor) GetMetadata(input string) ([]Metadata, error) {
	cmd := exec.Command(e.YtdlPath, "--default-search", e.DefaultSearch, "-j", input)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, newError(input, err)
	}

	var ret []Metadata
	dec := json.NewDecoder(&out)
	for dec.More() {
		var meta interface{}
		if err := dec.Decode(&meta); err != nil {
			return nil, newError(input, err)
		}

		if m, ok := meta.(map[string]interface{}); ok {
			ret = append(ret, m)
		} else {
			return nil, newError(input, errInvalidMetadata)
		}
	}
	return ret, nil
}

// Returns the best available audio-only format. If the given URL links directly
// to a media file, it just returns that URL.
func GetAudioURL(meta Metadata) (string, error) {
	// This function has a lot of code, but what it does is actually pretty
	// simple. We just have to do a lot to ensure that there are no integrity
	// issues with the metadata.
	// 'iSomething' stands for 'interface something' here.

	iExtractor, ok := meta["extractor"]
	if !ok {
		// Extractor not specified.
		return "", newError("", errInvalidMetadata)
	}
	if extractor, ok := iExtractor.(string); ok {
		if extractor == "generic" {
			url, ok := meta["url"]
			if u := url.(string); ok {
				return u, nil
			} else {
				return "", newError("", errors.New("unable to get any audio or video URL"))
			}
		}

		iFormats, ok := meta["formats"]
		if !ok {
			return "", newError("", errors.New("no format selection available and no raw URL specified"))
		}
		// Get the best audio format.
		if formats, ok := iFormats.([]interface{}); ok {
			// In youtube-dl, the last format is always the best one. Here we're
			// looking for the last (=best) format that contains no video.
			for i := len(formats) - 1; i >= 0; i-- {
				iFormat := formats[i]
				format, ok := iFormat.(map[string]interface{})
				if !ok {
					return "", newError("", errInvalidMetadata)
				}
				if format["vcodec"].(string) == "none" {
					return format["url"].(string), nil
				}
			}
			return "", newError("", errors.New("unable to find any audio-only format"))
		} else {
			return "", newError("", errInvalidMetadata)
		}
	} else {
		return "", newError("", errInvalidMetadata)
	}
}
