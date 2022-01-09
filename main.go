package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"trumpet/dca0"
	"trumpet/util"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"github.com/bwmarrin/dgvoice"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	texttospeechpb "google.golang.org/genproto/googleapis/cloud/texttospeech/v1"
)

////////////////////////////////
// Helper functions.
////////////////////////////////
// The first return value contains the voice channel ID, if it was found. If it
// was not found, it is set to "".
// The second return value indicates whether the voice channel was found.
func GetUserVoiceChannel(g *discordgo.Guild, userID string) (string, bool) {
	for _, vs := range g.VoiceStates {
		if vs.UserID == userID {
			return vs.ChannelID, true
		}
	}
	return "", false
}

////////////////////////////////
// Structs.
////////////////////////////////
type Playback struct {
	Track
	CmdCh  chan dca0.Command
	RespCh chan dca0.Response
	Paused bool
	Loop   bool // Whether playback is looping right now.
}

type Track struct {
	Title    string // Title, if any.
	Url      string // Short URL, for example from YouTube.
	MediaUrl string // Long URL of the associated media file.
}

// All methods of Client are thread safe, however manual locking is required
// when accessing any fields.
type Client struct {
	sync.RWMutex

	// The discordgo session.
	s *discordgo.Session

	// TextChannelID and VoiceChannelID indicate the current channels through
	// which the bot should send text / audio. They may be set to "".
	TextChannelID  string
	VoiceChannelID string

	// Current audio playback.
	Playback *Playback
	// Queue.
	Queue []*Track
}

func NewClient(s *discordgo.Session) *Client {
	return &Client{
		s: s,
	}
}

func SynthesizeSpeech(googleServiceAccount string, text string) []byte {
	b, err := ioutil.ReadFile(googleServiceAccount)
	if err != nil {
		logger.Fatal(fmt.Sprintf("unable to open google service account file: %s\n", err.Error()))
	}
	ctx := context.Background()
	c, err := texttospeech.NewClient(ctx, option.WithCredentialsJSON(b))
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to create client: %s", err.Error()))
	}
	defer c.Close()

	si := &texttospeechpb.SynthesisInput{
		InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
	}
	vsp := &texttospeechpb.VoiceSelectionParams{
		LanguageCode: "en-US",
		Name:         "en-US-Wavenet-F",
	}
	ac := &texttospeechpb.AudioConfig{
		AudioEncoding: texttospeechpb.AudioEncoding_OGG_OPUS,
		SpeakingRate:  1.5,
	}
	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input:       si,
		Voice:       vsp,
		AudioConfig: ac,
	}

	resp, err := c.SynthesizeSpeech(ctx, req)
	if err != nil {
		logger.Error(err.Error())
	}

	return resp.AudioContent
}

func (c *Client) Messagef(format string, a ...interface{}) {
	c.RLock()
	if c.TextChannelID == "" {
		fmt.Printf(format+"\n", a...)
	} else {
		c.s.ChannelMessageSend(c.TextChannelID, fmt.Sprintf(format, a...))
	}
	c.RUnlock()
}

// Updates the text channel and voice channel IDs. May set them to "" if there
// are none associated with the message.
func (c *Client) UpdateChannels(g *discordgo.Guild, m *discordgo.Message) {
	c.Lock()
	c.TextChannelID = m.ChannelID

	vc, _ := GetUserVoiceChannel(g, m.Author.ID)
	c.VoiceChannelID = vc
	c.Unlock()
}

func (c *Client) GetTextChannelID() string {
	c.RLock()
	ret := c.TextChannelID
	c.RUnlock()
	return ret
}

func (c *Client) GetVoiceChannelID() string {
	c.RLock()
	ret := c.TextChannelID
	c.RUnlock()
	return ret
}

// Returns a COPY of c.Playback. Modifications to the returned Playback struct
// are NOT preserved, as it is a copy.
func (c *Client) GetPlaybackInfo() (p Playback, ok bool) {
	c.RLock()
	ret := c.Playback
	c.RUnlock()
	if ret == nil {
		return Playback{}, false
	} else {
		return *ret, true
	}
}

func (c *Client) QueueLen() int {
	c.RLock()
	l := len(c.Queue)
	c.RUnlock()
	return l
}

// Similarly to GetPlaybackInfo, this function returns a COPY. Any modifications
// are NOT preserved.
// ok field returns false if the index is out of bounds.
func (c *Client) QueueAt(i int) (t Track, ok bool) {
	l := c.QueueLen()
	if i >= l || i < 0 {
		return Track{}, false
	}
	c.RLock()
	ret := c.Queue[i]
	c.RUnlock()
	return *ret, true
}

func (c *Client) QueuePushBack(t *Track) {
	c.Lock()
	c.Queue = append(c.Queue, t)
	c.Unlock()
}

func (c *Client) QueuePushFront(t *Track) {
	c.Lock()
	c.Queue = append([]*Track{t}, c.Queue...)
	c.Unlock()
}

func (c *Client) QueuePopFront() (t Track, ok bool) {
	t, ok = c.QueueAt(0)
	if ok {
		c.Lock()
		c.Queue = c.Queue[1:]
		c.Unlock()
	}
	return t, ok
}

// Deletes a single item at any position.
// Returns false if i was out of bounds.
func (c *Client) QueueDelete(i int) bool {
	if i >= c.QueueLen() {
		return false
	}
	c.Lock()
	c.Queue = append(c.Queue[:i], c.Queue[i+1:]...)
	c.Unlock()
	return true
}

// Swaps two items in the queue.
// Returns false if a or b is out of bounds.
func (c *Client) QueueSwap(a, b int) bool {
	if a == b {
		return true
	}
	l := c.QueueLen()
	if a >= l || b >= l || a < 0 || b < 0 {
		return false
	}
	c.Lock()
	c.Queue[a], c.Queue[b] = c.Queue[b], c.Queue[a]
	c.Unlock()
	return true
}

func (c *Client) QueueClear() {
	c.Lock()
	c.Queue = nil
	c.Unlock()
}

// GetAudioFile checks the audio cache or creates the file.
func GetAudioFile(messages []string, userid string) error {

	joinPath := fmt.Sprintf("%s_join.ogg", filepath.Join(cfg.UserAudioPath, userid))
	partPath := fmt.Sprintf("%s_leave.ogg", filepath.Join(cfg.UserAudioPath, userid))

	_, err := os.OpenFile(joinPath, os.O_RDONLY, 0640)
	if errors.Is(err, os.ErrNotExist) {
		logger.Warn("Join file doesn't exist, creating...")
		joinGreet := SynthesizeSpeech(cfg.GoogleServiceAccountCredentials, messages[0])
		err = ioutil.WriteFile(joinPath, joinGreet, 0640)
		if err != nil {
			logger.Error("Failed to write file",
				zap.Error(err),
			)
		}
	} else {
		return err
	}

	_, err = os.OpenFile(partPath, os.O_RDONLY, 0640)
	if errors.Is(err, os.ErrNotExist) {
		logger.Warn("Leave file doesn't exist, creating...")
		partGreet := SynthesizeSpeech(cfg.GoogleServiceAccountCredentials, messages[1])
		err = ioutil.WriteFile(partPath, partGreet, 0640)
		if err != nil {
			logger.Error("Failed to write file:",
				zap.Error(err),
			)
		}
	} else {
		return err
	}

	return nil
}

////////////////////////////////
// Global variables.
////////////////////////////////
var clients map[string]*Client // Guild ID to client
var mClients sync.Mutex

var cfg Config
var logger *zap.Logger

////////////////////////////////
// Main program.
////////////////////////////////
func main() {
	logger, _ = zap.NewProduction()
	defer logger.Sync()
	if err := ReadConfig(&cfg); err != nil {
		fmt.Println(err)
		if err := WriteDefaultConfig(); err != nil {
			fmt.Println("Failed to create the default configuration file:", err)
			return
		}
		fmt.Println("Wrote the default configuration to " + configFile + ".")
		fmt.Println("You will have to manually configure the token by editing " + configFile + ".")
		return
	}

	if cfg.Token == tokenDefaultString {
		fmt.Println("Please set your bot token in " + configFile + " first.")
		return
	}

	// Check if all binary dependencies are installed correctly.
	const notInstalledErrMsg = "Unable to find %s in the specified path '%s', please make sure it's installed correctly.\nYou can manually set its path by editing %s\n"
	if !util.CheckInstalled(cfg.YtdlPath, "--version") {
		fmt.Printf(notInstalledErrMsg, "youtube-dl", cfg.YtdlPath, configFile)
		return
	}
	if !util.CheckInstalled(cfg.FfmpegPath, "-version") {
		fmt.Printf(notInstalledErrMsg, "ffmpeg", cfg.FfmpegPath, configFile)
		return
	}

	// Initialize client map.
	clients = make(map[string]*Client)

	// Initialize bot.
	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		fmt.Println("Error creating Discord session:", err)
		return
	}

	dg.AddHandler(ready)
	// dg.AddHandler(banAdd)
	dg.AddHandler(messageCreate)
	dg.AddHandler(announce)

	// What information we need about guilds.
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildBans
	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord session:", err)
		return
	}
	logger.Info("Opened Discord websocket session.")

	// Wait here until Ctrl+c or other term signal is received.
	fmt.Println("Bot is now running. Press Ctrl+c to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	logger.Info("Signal received, closing Discord session.")
	fmt.Println("Signal received, closing Discord session.")

	// Cleanly close down the Discord session.
	dg.Close()
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	u := s.State.User
	log_string := fmt.Sprintf("Logged in to Discord as " + u.Username + "#" + u.Discriminator + ". Discord UID: " + u.ID + ".")
	logger.Info(log_string,
		zap.String("username", u.Username),
		zap.String("discriminator", u.Discriminator),
		zap.String("id", u.ID),
	)
	fmt.Println("Logged in as", u.Username+"#"+u.Discriminator+".")
	s.UpdateListeningStatus(cfg.Prefix + "help")
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself.
	if m.Author.ID == s.State.User.ID {
		return
	}

	var g *discordgo.Guild
	g, err := s.State.Guild(m.GuildID)
	if err != nil {
		// Could not find guild.
		s.ChannelMessageSend(m.ChannelID, "This bot only works in guilds (servers).")
		return
	}

	var c *Client
	mClients.Lock()
	{
		var ok bool
		if c, ok = clients[m.GuildID]; !ok {
			c = NewClient(s)
			clients[m.GuildID] = c
		}
	}
	mClients.Unlock()
	// Update the text and voice channels associated with the client.
	c.UpdateChannels(g, m.Message)

	args, ok := CmdGetArgs(m.Content)
	if !ok {
		// Not a command.
		return
	}

	if len(args) == 0 {
		c.Messagef("No command specified. Type `%shelp` for help.", cfg.Prefix)
		return
	}

	switch args[0] {
	case "help":
		logger.Info(fmt.Sprintf("help command called by " + m.Author.Username + "#" + m.Author.Discriminator + " in channel " + m.ChannelID + " on server " + m.GuildID + "."))
		commandHelp(c)
	case "play":
		logger.Info(fmt.Sprintf("play command called by "+m.Author.Username+"#"+m.Author.Discriminator+" in channel "+m.ChannelID+" on server "+m.GuildID+"."),
			zap.String("args", fmt.Sprintf("%s", args[1:])),
		)
		commandPlay(s, g, c, args[1:])
	case "seek":
		logger.Info(fmt.Sprintf("seek command called by "+m.Author.Username+"#"+m.Author.Discriminator+" in channel "+m.ChannelID+" on server "+m.GuildID+"."),
			zap.String("args", fmt.Sprintf("%s", args[1:])),
		)
		commandSeek(c, args[1:])
	case "pos":
		logger.Info(fmt.Sprintf("pos command called by "+m.Author.Username+"#"+m.Author.Discriminator+" in channel "+m.ChannelID+" on server "+m.GuildID+"."),
			zap.String("args", fmt.Sprintf("%s", args[1:])),
		)
		commandPos(c)
	case "loop":
		logger.Info(fmt.Sprintf("loop command called by " + m.Author.Username + "#" + m.Author.Discriminator + " in channel " + m.ChannelID + " on server " + m.GuildID + "."))
		commandLoop(c)
	case "add":
		logger.Info(fmt.Sprintf("add command called by "+m.Author.Username+"#"+m.Author.Discriminator+" in channel "+m.ChannelID+" on server "+m.GuildID+"."),
			zap.String("args", fmt.Sprintf("%s", args[1:])),
		)
		commandAdd(c, args[1:], false)
	case "queue":
		logger.Info(fmt.Sprintf("queue command called by " + m.Author.Username + "#" + m.Author.Discriminator + " in channel " + m.ChannelID + " on server " + m.GuildID + "."))
		commandQueue(c)
	case "pause":
		logger.Info(fmt.Sprintf("pause command called by " + m.Author.Username + "#" + m.Author.Discriminator + " in channel " + m.ChannelID + " on server " + m.GuildID + "."))
		commandPause(c)
	case "stop":
		logger.Info(fmt.Sprintf("stop command called by " + m.Author.Username + "#" + m.Author.Discriminator + " in channel " + m.ChannelID + " on server " + m.GuildID + "."))
		commandStop(c)
	case "skip":
		logger.Info(fmt.Sprintf("skip command called by " + m.Author.Username + "#" + m.Author.Discriminator + " in channel " + m.ChannelID + " on server " + m.GuildID + "."))
		commandSkip(c)
	case "delete":
		logger.Info(fmt.Sprintf("delete command called by "+m.Author.Username+"#"+m.Author.Discriminator+" in channel "+m.ChannelID+" on server "+m.GuildID+"."),
			zap.String("args", fmt.Sprintf("%s", args[1:])),
		)
		commandDelete(c, args[1:])
	case "shuffle":
		logger.Info(fmt.Sprintf("shuffle command called by " + m.Author.Username + "#" + m.Author.Discriminator + " in channel " + m.ChannelID + " on server " + m.GuildID + "."))
		commandShuffle(c)
	}
}

func announce(s *discordgo.Session, event *discordgo.VoiceStateUpdate) {
	// Ignore messages from the bot itself.
	if event.UserID == s.State.User.ID {
		return
	}

	// Get the member object for the user.
	member, err := s.GuildMember(event.GuildID, event.UserID)
	if err != nil {
		logger.Error("Error: Failed to get member " + event.UserID + ".")
		return
	}

	// Check if it's a user on the ignore list.
	for _, ignore := range cfg.IgnoreList {
		if member.User.Username == ignore {
			return
		}
	}

	// Initial voice event. Also cache join/part audio for user if it doesn't exist.
	logger.Debug("Voice event for user: " + member.User.Username + "#" + member.User.Discriminator + ".")
	join := fmt.Sprintf("%s joined.", member.User.Username)
	part := fmt.Sprintf("%s left.", member.User.Username)
	msgs := []string{join, part}

	err = GetAudioFile(msgs, member.User.ID)
	if err != nil {
		logger.Error("Error: Failed to get audio file for user " + member.User.Username + "#" + member.User.Discriminator + ".")
	}

	s.RLock()
	vc := s.VoiceConnections[event.GuildID]
	s.RUnlock()
	if vc == nil {
		vc, err := s.ChannelVoiceJoin(event.GuildID, event.ChannelID, false, true)
		if err != nil {
			logger.Sugar().Errorf("Error joining voice channel: %s.", err)
			return
		}
		defer vc.Disconnect()
		s.Lock()
		s.VoiceConnections[event.GuildID] = vc
		s.Unlock()
	}
	// vc.Lock()
	// vc.LogLevel = discordgo.LogDebug
	// vc.Unlock()

	// Try to determine the type of event.
	if err != nil {
		logger.Error("Error: Failed to get voice state of user " + event.UserID + ".")
		return
	}
	if event.BeforeUpdate == nil {
		logger.Info("User has joined voice channel: " + member.User.Username + "#" + member.User.Discriminator + ".")
		time.Sleep(1250 * time.Millisecond)
		// TODO: Make this a configurable option.
		dgvoice.PlayAudioFile(s.VoiceConnections[event.GuildID], "trumpet.opus", make(<-chan bool))
		dgvoice.PlayAudioFile(s.VoiceConnections[event.GuildID], filepath.Join(cfg.UserAudioPath, member.User.ID)+"_join.ogg", make(<-chan bool))
		logger.Debug("Event doesn't have a BeforeUpdate",
			zap.String("Member", fmt.Sprintf("%s#%s", member.User.Username, member.User.Discriminator)),
			zap.String("Channel", event.ChannelID),
			zap.String("CurrentState", fmt.Sprintf("%#v", event.VoiceState)),
			zap.String("EventStruct for debugging", fmt.Sprintf("%+v", event)),
		)
		return
	}
	// Ignore Server/Self Mute/Deafen events.
	if event.BeforeUpdate.Deaf != event.VoiceState.Deaf {
		return
	} else if event.BeforeUpdate.SelfDeaf != event.VoiceState.SelfDeaf {
		return
	} else if event.BeforeUpdate.Mute != event.VoiceState.Mute {
		return
	} else if event.BeforeUpdate.SelfMute != event.VoiceState.SelfMute {
		return
	}

	// Get current bot channel id
	botChannel, err := s.State.VoiceState(event.GuildID, s.State.User.ID)
	if err != nil {
		logger.Error("Error: The bot is not in a voice channel! Joining... " + event.ChannelID)
		s.ChannelVoiceJoin(event.GuildID, event.ChannelID, false, true)
		return
	}
	// Ignore messages from voice channels the bot is not in.
	if event.ChannelID != botChannel.ChannelID {
		logger.Info("User has left voice channel: " + member.User.Username + "#" + member.User.Discriminator + ".")
		dgvoice.PlayAudioFile(s.VoiceConnections[event.GuildID], filepath.Join(cfg.UserAudioPath, member.User.ID)+"_leave.ogg", make(<-chan bool))
	}

	logger.Debug("DEBUG: Events here are screenshare-related or otherwise unknown.")
	logger.Debug("Event contains a BeforeUpdate",
		zap.String("Member", fmt.Sprintf("%s#%s", member.User.Username, member.User.Discriminator)),
		zap.String("Channel", event.ChannelID),
		zap.String("BeforeState", fmt.Sprintf("%#v", event.BeforeUpdate)),
		zap.String("CurrentState", fmt.Sprintf("%#v", event.VoiceState)),
		zap.String("EventStruct for debugging", fmt.Sprintf("%+v", event)),
	)
}
