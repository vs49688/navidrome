package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kr/pretty"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

type configOptions struct {
	ConfigFile              string
	Address                 string
	Port                    int
	MusicFolder             string
	DataFolder              string
	DbPath                  string
	LogLevel                string
	ScanInterval            time.Duration
	ScanSchedule            string
	SessionTimeout          time.Duration
	BaseURL                 string
	UILoginBackgroundURL    string
	EnableTranscodingConfig bool
	EnableDownloads         bool
	EnableExternalServices  bool
	TranscodingCacheSize    string
	ImageCacheSize          string
	AutoImportPlaylists     bool
	PlaylistsPath           string

	SearchFullString       bool
	RecentlyAddedByModTime bool
	IgnoredArticles        string
	IndexGroups            string
	ProbeCommand           string
	CoverArtPriority       string
	CoverJpegQuality       int
	UIWelcomeMessage       string
	EnableGravatar         bool
	EnableFavourites       bool
	EnableStarRating       bool
	EnableUserEditing      bool
	DefaultTheme           string
	EnableCoverAnimation   bool
	GATrackingID           string
	EnableLogRedacting     bool
	AuthRequestLimit       int
	AuthWindowLength       time.Duration
	PasswordEncryptionKey  string
	ReverseProxyUserHeader string
	ReverseProxyWhitelist  string

	Scanner scannerOptions

	Agents       string
	LastFM       lastfmOptions
	Spotify      spotifyOptions
	ListenBrainz listenBrainzOptions

	// DevFlags. These are used to enable/disable debugging and incomplete features
	DevLogSourceLine           bool
	DevLogLevels               map[string]string
	DevAutoCreateAdminPassword string
	DevAutoLoginUsername       string
	DevPreCacheAlbumArtwork    bool
	DevFastAccessCoverArt      bool
	DevActivityPanel           bool
	DevEnableShare             bool
	DevSidebarPlaylists        bool
	DevEnableBufferedScrobble  bool
	DevShowArtistPage          bool
	DevDisableGenreCache       bool
}

type scannerOptions struct {
	Extractor       string
	GenreSeparators string
}

type lastfmOptions struct {
	Enabled  bool
	ApiKey   string
	Secret   string
	Language string
}

type spotifyOptions struct {
	ID     string
	Secret string
}

type listenBrainzOptions struct {
	Enabled bool
}

var (
	Server = &configOptions{}
	hooks  []func()
)

func LoadFromFile(confFile string) {
	viper.SetConfigFile(confFile)
	Load()
}

func Load() {
	err := viper.Unmarshal(&Server)
	if err != nil {
		fmt.Println("FATAL: Error parsing config:", err)
		os.Exit(1)
	}
	err = os.MkdirAll(Server.DataFolder, os.ModePerm)
	if err != nil {
		fmt.Println("FATAL: Error creating data path:", "path", Server.DataFolder, err)
		os.Exit(1)
	}
	Server.ConfigFile = viper.GetViper().ConfigFileUsed()
	if Server.DbPath == "" {
		Server.DbPath = filepath.Join(Server.DataFolder, consts.DefaultDbPath)
	}

	log.SetLevelString(Server.LogLevel)
	log.SetLogLevels(Server.DevLogLevels)
	log.SetLogSourceLine(Server.DevLogSourceLine)
	log.SetRedacting(Server.EnableLogRedacting)

	if err := validateScanSchedule(); err != nil {
		os.Exit(1)
	}

	// Print current configuration if log level is Debug
	if log.CurrentLevel() >= log.LevelDebug {
		prettyConf := pretty.Sprintf("Loaded configuration from '%s': %# v", Server.ConfigFile, Server)
		if Server.EnableLogRedacting {
			prettyConf = log.Redact(prettyConf)
		}
		fmt.Println(prettyConf)
	}

	if !Server.EnableExternalServices {
		disableExternalServices()
	}

	// Call init hooks
	for _, hook := range hooks {
		hook()
	}
}

func disableExternalServices() {
	log.Info("All external integrations are DISABLED!")
	Server.LastFM.Enabled = false
	Server.Spotify.ID = ""
	Server.ListenBrainz.Enabled = false
	Server.Agents = ""
	if Server.UILoginBackgroundURL == consts.DefaultUILoginBackgroundURL {
		Server.UILoginBackgroundURL = consts.DefaultUILoginBackgroundURLOffline
	}
}

func validateScanSchedule() error {
	if Server.ScanInterval != -1 {
		log.Warn("ScanInterval is DEPRECATED. Please use ScanSchedule. See docs at https://navidrome.org/docs/usage/configuration-options/")
		if Server.ScanSchedule != "@every 1m" {
			log.Error("You cannot specify both ScanInterval and ScanSchedule, ignoring ScanInterval")
		} else {
			if Server.ScanInterval == 0 {
				Server.ScanSchedule = ""
			} else {
				Server.ScanSchedule = fmt.Sprintf("@every %s", Server.ScanInterval)
			}
			log.Warn("Setting ScanSchedule", "schedule", Server.ScanSchedule)
		}
	}
	if Server.ScanSchedule == "0" || Server.ScanSchedule == "" {
		Server.ScanSchedule = ""
		return nil
	}
	if _, err := time.ParseDuration(Server.ScanSchedule); err == nil {
		Server.ScanSchedule = "@every " + Server.ScanSchedule
	}
	c := cron.New()
	_, err := c.AddFunc(Server.ScanSchedule, func() {})
	if err != nil {
		log.Error("Invalid ScanSchedule. Please read format spec at https://pkg.go.dev/github.com/robfig/cron#hdr-CRON_Expression_Format", "schedule", Server.ScanSchedule, err)
	}
	return err
}

// AddHook is used to register initialization code that should run as soon as the config is loaded
func AddHook(hook func()) {
	hooks = append(hooks, hook)
}

func init() {
	viper.SetDefault("musicfolder", filepath.Join(".", "music"))
	viper.SetDefault("datafolder", ".")
	viper.SetDefault("loglevel", "info")
	viper.SetDefault("address", "0.0.0.0")
	viper.SetDefault("port", 4533)
	viper.SetDefault("sessiontimeout", consts.DefaultSessionTimeout)
	viper.SetDefault("scaninterval", -1)
	viper.SetDefault("scanschedule", "@every 1m")
	viper.SetDefault("baseurl", "")
	viper.SetDefault("uiloginbackgroundurl", consts.DefaultUILoginBackgroundURL)
	viper.SetDefault("enabletranscodingconfig", false)
	viper.SetDefault("transcodingcachesize", "100MB")
	viper.SetDefault("imagecachesize", "100MB")
	viper.SetDefault("autoimportplaylists", true)
	viper.SetDefault("playlistspath", consts.DefaultPlaylistsPath)
	viper.SetDefault("enabledownloads", true)
	viper.SetDefault("enableexternalservices", true)

	// Config options only valid for file/env configuration
	viper.SetDefault("searchfullstring", false)
	viper.SetDefault("recentlyaddedbymodtime", false)
	viper.SetDefault("ignoredarticles", "The El La Los Las Le Les Os As O A")
	viper.SetDefault("indexgroups", "A B C D E F G H I J K L M N O P Q R S T U V W X-Z(XYZ) [Unknown]([)")
	viper.SetDefault("probecommand", "ffmpeg %s -f ffmetadata")
	viper.SetDefault("coverartpriority", "embedded, cover.*, folder.*, front.*")
	viper.SetDefault("coverjpegquality", 75)
	viper.SetDefault("uiwelcomemessage", "")
	viper.SetDefault("enablegravatar", false)
	viper.SetDefault("enablefavourites", true)
	viper.SetDefault("enablestarrating", true)
	viper.SetDefault("enableuserediting", true)
	viper.SetDefault("defaulttheme", "Dark")
	viper.SetDefault("enablecoveranimation", true)
	viper.SetDefault("gatrackingid", "")
	viper.SetDefault("enablelogredacting", true)
	viper.SetDefault("authrequestlimit", 5)
	viper.SetDefault("authwindowlength", 20*time.Second)
	viper.SetDefault("passwordencryptionkey", "")

	viper.SetDefault("reverseproxyuserheader", "Remote-User")
	viper.SetDefault("reverseproxywhitelist", "")

	viper.SetDefault("scanner.extractor", consts.DefaultScannerExtractor)
	viper.SetDefault("scanner.genreseparators", ";/,")

	viper.SetDefault("agents", "lastfm,spotify")
	viper.SetDefault("lastfm.enabled", true)
	viper.SetDefault("lastfm.language", "en")
	viper.SetDefault("lastfm.apikey", consts.LastFMAPIKey)
	viper.SetDefault("lastfm.secret", consts.LastFMAPISecret)
	viper.SetDefault("spotify.id", "")
	viper.SetDefault("spotify.secret", "")
	viper.SetDefault("listenbrainz.enabled", true)

	// DevFlags. These are used to enable/disable debugging and incomplete features
	viper.SetDefault("devlogsourceline", false)
	viper.SetDefault("devautocreateadminpassword", "")
	viper.SetDefault("devautologinusername", "")
	viper.SetDefault("devprecachealbumartwork", false)
	viper.SetDefault("devfastaccesscoverart", false)
	viper.SetDefault("devactivitypanel", true)
	viper.SetDefault("devenableshare", false)
	viper.SetDefault("devenablebufferedscrobble", true)
	viper.SetDefault("devsidebarplaylists", true)
	viper.SetDefault("devshowartistpage", true)
}

func InitConfig(cfgFile string) {
	cfgFile = getConfigFile(cfgFile)
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in local directory with name "navidrome" (without extension).
		viper.AddConfigPath(".")
		viper.SetConfigName("navidrome")
	}

	_ = viper.BindEnv("port")
	viper.SetEnvPrefix("ND")
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if viper.ConfigFileUsed() != "" && err != nil {
		fmt.Println("FATAL: Navidrome could not open config file: ", err)
		os.Exit(1)
	}
}

func getConfigFile(cfgFile string) string {
	if cfgFile != "" {
		return cfgFile
	}
	return os.Getenv("ND_CONFIGFILE")
}
