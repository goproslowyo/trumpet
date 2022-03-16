package main

import (
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"strings"
	"time"

	"trumpet/dca0"
	"trumpet/ytdl"

	"github.com/bwmarrin/discordgo"
)

////////////////////////////////
// Helper functions.
////////////////////////////////
func generateHelpMsg() string {
	// Align all commands nicely so that the descriptions are in the same
	// column.
	longestCmd := 0
	var cmds, descs []string
	addCmd := func(cmd, desc string) {
		if len(cmd) > longestCmd {
			longestCmd = len(cmd)
		}
		cmds = append(cmds, cmd)
		descs = append(descs, desc)
	}
	addCmd("help", "show this page")
	addCmd("play <URL|query>", "play audio from a URL or a youtube search query")
	addCmd("play", "start playing queue/resume playback")
	addCmd("seek <time>", "seek to the specified time (format: mm:ss or seconds)")
	addCmd("pos", "get the current playback time")
	addCmd("loop", "start/stop looping the current track")
	addCmd("add <URL|query>", "add a URL or a youtube search query to the queue; note: be patient when adding large playlists")
	addCmd("queue", "print the current queue; used to obtain track IDs for some other commands")
	addCmd("pause", "pause playback")
	addCmd("stop", "clear playlist and stop playback")
	addCmd("skip", "skip the current track")
	addCmd("delete <ID|ID-ID>...", "delete one or multiple tracks from the queue")
	addCmd("swap <ID> <ID>", "swap the position of two tracks in the queue")
	addCmd("shuffle", "shuffle all items in the current queue")

	var msg strings.Builder
	msg.WriteString("Commands:\n")
	for i := range cmds {
		msg.WriteString("\u2022 `" + cfg.Prefix + cmds[i])
		msg.WriteString(strings.Repeat(" ", longestCmd-len(cmds[i])))
		msg.WriteString(" - " + descs[i] + "`\n")
	}
	return msg.String()
}

// Converts seconds to string in format mm:ss.
func secsToMinsSecs(secs int) string {
	return fmt.Sprintf("%02d:%02d", secs/60, secs%60)
}

// Removes/replaces some characters with special formatting meaning in discord
// messages (for example *).
func dcSanitize(in string) string {
	in = strings.ReplaceAll(in, "*", "\\*")
	in = strings.ReplaceAll(in, "~~", "\\~\\~")
	in = strings.ReplaceAll(in, "`", "\\`")
	return strings.ReplaceAll(in, "||", "\\|\\|")
}

////////////////////////////////
// Global variables.
////////////////////////////////
var helpMsg string

////////////////////////////////
// The actual commands.
////////////////////////////////
func commandHelp(c *Client) {
	// We're not generating the help message in the var declaration because
	// generateHelpMsg() relies on the config which hasn't been read at that
	// point.
	if helpMsg == "" {
		helpMsg = generateHelpMsg()
	}
	c.Messagef("%s", helpMsg)
}

func commandPlay(s *discordgo.Session, g *discordgo.Guild, c *Client, args []string) {
	var playbackActive bool
	{
		var playback Playback
		if playback, playbackActive = c.GetPlaybackInfo(); playbackActive {
			if playback.Paused {
				c.Messagef("Resuming playback.")
				playback.CmdCh <- dca0.CommandResume{}
				c.Lock()
				c.Playback.Paused = false
				c.Unlock()
				return
			}
		}
	}

	if c.QueueLen() == 0 && len(args) == 0 {
		c.Messagef("Nothing in queue. Please add an item to the queue or specify a URL or a youtube search query.")
		return
	}

	if len(args) > 0 {
		// Add the current track/playlist in place.
		commandAdd(c, args, false)
		// We only want one player active at once.
		if playbackActive {
			return
		}
	}

	if c.VoiceChannelID == "" {
		c.Messagef("I don't know which voice channel to join.")
		return
	}

	vc, err := s.ChannelVoiceJoin(g.ID, c.VoiceChannelID, false, true)
	if err != nil {
		c.Messagef("Error joining voice channel: %s.", err)
		return
	}
	defer vc.Disconnect()

	// Set playback to nothing once we're done or if an error occurs.
	defer func() {
		c.Lock()
		c.Playback = nil
		c.Unlock()
	}()

	// Play the queue.
	for c.QueueLen() > 0 {
		track, _ := c.QueuePopFront()
		mediaUrl := track.MediaUrl
		c.Messagef("Playing: %s.\n", dcSanitize(track.Title))

		// Set up dca0 encoder.
		dcaOpts := dca0.GetDefaultOptions(cfg.FfmpegPath)
		enc, err := dca0.NewEncoder(dcaOpts)
		if err != nil {
			c.Messagef("Error: %s.", err)
			return
		}

		// Set up audio playback.
		c.Lock()
		c.Playback = &Playback{
			CmdCh:  make(chan dca0.Command),
			RespCh: make(chan dca0.Response),
			Track:  track,
		}
		c.Unlock()
		// We just set the playback info so we don't have to check if it's there.
		playback, _ := c.GetPlaybackInfo()
		errCh := make(chan error)
		// Start downloading and sending audio data.
		vc.Speaking(true)
		go func() {
			enc.GetOpusFrames(mediaUrl, dcaOpts, vc.OpusSend, errCh, playback.CmdCh, playback.RespCh)
			close(errCh)
		}()
		// Process errors: Get and print the first error put out by the extractor.
		err = nil
		for e := range errCh {
			if e != nil && err == nil {
				err = e
			}
		}
		if err != nil {
			c.Messagef("Playback error: %s.", err)
			return
		}
		// Done with this song.
		playback, _ = c.GetPlaybackInfo()
	}
	c.Messagef("Done playing queue.")
}

// If inPlace is set to true, the track will be added to the front and replace
// the currently playing one. If inPlace is set to true when dealing with a
// playlist, the entire queue is replaced with that playlist.
func commandAdd(c *Client, args []string, inPlace bool) {
	if len(args) < 1 {
		c.Messagef("Please specify a URL or a youtube search query.")
		return
	}

	// URL or search query.
	input := strings.Join(args, " ")

	// TODO: This is some very shitty detection for if we're dealing with a
	// playlist.
	if strings.HasPrefix(path.Base(input), "playlist") {
		c.Messagef("Long playlists may take a while to add, please be patient.")
	}

	ytdlEx := ytdl.NewExtractor(cfg.YtdlPath)

	meta, err := ytdlEx.GetMetadata(input)
	if err != nil {
		c.Messagef("Error getting audio metadata: %s.", err)
		return
	}

	var plural string
	if len(meta) != 1 {
		plural = "s"
	}
	c.Messagef("Adding %d track%s to queue.", len(meta), plural)

	isPlaylist := len(meta) > 1
	if inPlace && isPlaylist {
		c.QueueClear()
	}

	for _, m := range meta {
		title, titleOk := m["title"]
		webpageUrl, webpageUrlOk := m["webpage_url"]
		if !(titleOk && webpageUrlOk) {
			c.Messagef("Error getting video metadata: title=%t, url=%t.", titleOk, webpageUrlOk)
			return
		}

		mediaUrl, err := ytdl.GetAudioURL(m)
		if err != nil {
			c.Messagef("Error getting URL: %s.", err)
			return
		}

		track := &Track{
			Title:    title.(string),
			Url:      webpageUrl.(string),
			MediaUrl: mediaUrl,
		}
		if inPlace && !isPlaylist {
			// To replace the currently playing track (if one is currently
			// playing), insert the new one at Queue[0] and skip the current
			// one.
			c.QueuePushFront(track)
			if playback, ok := c.GetPlaybackInfo(); ok {
				playback.CmdCh <- dca0.CommandStop{}
			}
		} else {
			c.QueuePushBack(track)
		}
	}
}

func commandQueue(c *Client) {
	playback, playbackOk := c.GetPlaybackInfo()
	ql := c.QueueLen()
	if ql == 0 && !playbackOk {
		c.Messagef("Queue is empty.")
		return
	}

	const maxLines = 15
	var msg strings.Builder
	// flush() writes the string buffer into a new Discord message, then clears
	// the buffer.
	flush := func() {
		c.Messagef("%s", msg.String())
		msg.Reset()
		msg.WriteString("\u2800\n")
	}
	msg.WriteString("\u2800\n")
	if playbackOk {
		var loop string
		if playback.Loop {
			loop = " [LOOP]"
		}
		msg.WriteString(fmt.Sprintf("PLAYING: " + dcSanitize(playback.Track.Title) + loop + "\n"))
	}
	for i := 0; i < ql; i++ {
		t, _ := c.QueueAt(i)
		msg.WriteString(fmt.Sprintf("%02d. %s\n", i+1, dcSanitize(t.Title)))
		// Only send a maximum of 15 lines at a time.
		if (i+1)%maxLines == 0 && i != ql-1 {
			flush()
		}
	}
	flush()
}

func commandSeek(c *Client, args []string) {
	playback, ok := c.GetPlaybackInfo()
	if !ok {
		c.Messagef("Not playing anything.")
		return
	}
	const invalidFormat = "Please specify where to seek, either in seconds or in the format of mm:ss."
	if len(args) == 0 {
		c.Messagef(invalidFormat)
		return
	}
	splits := strings.Split(args[0], ":")
	var sMins, sSecs string
	if len(splits) == 2 {
		sMins, sSecs = splits[0], splits[1]
	} else if len(splits) == 1 {
		sMins, sSecs = "", splits[0]
	} else {
		c.Messagef(invalidFormat)
		return
	}
	var mins, secs int64
	var err error
	if sMins != "" {
		mins, err = strconv.ParseInt(sMins, 10, 32)
		if err != nil {
			c.Messagef(invalidFormat)
			return
		}
	}
	secs, err = strconv.ParseInt(sSecs, 10, 32)
	if err != nil {
		c.Messagef(invalidFormat)
		return
	}
	secs = 60*mins + secs
	c.Messagef("Seeking to %s.", secsToMinsSecs(int(secs)))
	playback.CmdCh <- dca0.CommandSeek(secs)
}

func commandPos(c *Client) {
	playback, ok := c.GetPlaybackInfo()
	if !ok {
		c.Messagef("Not playing anything.")
		return
	}
	var sTime, sDur string
	// Get current playback time.
	playback.CmdCh <- dca0.CommandGetPlaybackTime{}
	respTime := <-playback.RespCh
	if t, ok := respTime.(dca0.ResponsePlaybackTime); ok {
		sTime = secsToMinsSecs(int(t))
	} else {
		c.Messagef("Error receiving response: invalid type.")
		return
	}
	// Attempt to get duration.
	playback.CmdCh <- dca0.CommandGetDuration{}
	respDur := <-playback.RespCh
	switch d := respDur.(type) {
	case dca0.ResponseDurationUnknown:
		sDur = "??:??"
	case dca0.ResponseDuration:
		sDur = secsToMinsSecs(int(d))
	default:
		c.Messagef("Error receiving response: invalid type.")
		return
	}

	c.Messagef("Current playback position: %s / %s.", sTime, sDur)
}

func commandLoop(c *Client) {
	playback, ok := c.GetPlaybackInfo()
	if !ok {
		c.Messagef("Not playing anything.")
		return
	}

	if playback.Loop {
		playback.CmdCh <- dca0.CommandStopLooping{}
		c.Messagef("Looping disabled.")
	} else {
		playback.CmdCh <- dca0.CommandStartLooping{}
		c.Messagef("Looping enabled.")
	}
	c.Lock()
	c.Playback.Loop = !playback.Loop
	c.Unlock()
}

func commandStop(c *Client) {
	playback, ok := c.GetPlaybackInfo()
	if !ok {
		c.Messagef("Not playing anything.")
		return
	}
	c.Messagef("Stopping playback.")
	c.QueueClear()
	playback.CmdCh <- dca0.CommandStop{}
}

func commandSkip(c *Client) {
	playback, ok := c.GetPlaybackInfo()
	if !ok {
		c.Messagef("Not playing anything.")
		return
	}
	c.Messagef("Skipping current track.")
	playback.CmdCh <- dca0.CommandStop{}
}

func commandPause(c *Client) {
	playback, ok := c.GetPlaybackInfo()
	if !ok {
		c.Messagef("Not playing anything.")
		return
	}
	if playback.Paused {
		c.Messagef("Already paused.")
	} else {
		c.Messagef("Pausing playback.")
		playback.CmdCh <- dca0.CommandPause{}
		c.Lock()
		c.Playback.Paused = true
		c.Unlock()
	}
}

func commandDelete(c *Client, args []string) {
	if len(args) < 1 {
		c.Messagef("Please specify which item(s) to delete from the queue. IDs can be obtained with %squeue.", cfg.Prefix)
		return
	}

	toDel := make(map[int]struct{})
	for _, arg := range args {
		splits := strings.Split(arg, "-")
		switch len(splits) {
		case 1, 2:
			ids := [2]int{-1, -1}
			for i, s := range splits {
				id, err := strconv.ParseInt(s, 10, 32)
				if err != nil {
					c.Messagef("Invalid format: %s.", arg)
					return
				}
				id--
				if id < 0 || int(id) >= c.QueueLen() {
					c.Messagef("Index out of bounds: %s.", arg)
					return
				}
				ids[i] = int(id)
			}
			if ids[1] == -1 {
				toDel[ids[0]] = struct{}{}
			} else {
				if ids[0] > ids[1] {
					c.Messagef("The first id of the range must be not be larger: %s.", arg)
					return
				}
				for i := ids[0]; i <= ids[1]; i++ {
					toDel[i] = struct{}{}
				}
			}
		default:
			c.Messagef("Invalid format: %s.", arg)
			return
		}
	}

	var newQueue []*Track
	queueLen := c.QueueLen()
	for i := 0; i < queueLen; i++ {
		if _, del := toDel[i]; !del {
			c.RLock()
			newQueue = append(newQueue, c.Queue[i])
			c.RUnlock()
		}
	}
	c.Lock()
	c.Queue = newQueue
	c.Unlock()
	c.Messagef("Successfully deleted %d items.", len(toDel))
}

func commandShuffle(c *Client) {
	rand.Seed(time.Now().Unix())
	queueLen := c.QueueLen()
	c.Lock()
	rand.Shuffle(queueLen, func(a, b int) {
		c.Queue[a], c.Queue[b] = c.Queue[b], c.Queue[a]
	})
	c.Unlock()
	c.Messagef("Successfully shuffled %d items.", queueLen)
}

func commandJoin(s *discordgo.Session, g *discordgo.Guild, c *Client, m *discordgo.MessageCreate) {
	// Get the voice channel the user is in (if any), otherwise let's bail
	if c.VoiceChannelID == "" {
		logger.Info("No VoiceChannelID associated with message")
		return
	}

	c.RLock()
	channelId := c.VoiceChannelID
	c.RUnlock()

	guildId := g.ID
	logger.Info(fmt.Sprintf("Attempting to join voice channel %s", channelId))

	_, err := s.ChannelVoiceJoin(guildId, channelId, false, true)
	if err != nil {
		logger.Sugar().Errorf("Error joining voice channel: %s.", err)
		return
	}
}
