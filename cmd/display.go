package cmd

import (
	"fmt"
	"image"
	"image/color"
	"net/http"
	"os"
	"reflect"
	"sptlrx/config"
	"sptlrx/lyrics"
	"sptlrx/pool"
	"sptlrx/services/spotify"
	custom "sptlrx/theme"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
	"github.com/nfnt/resize"
	"github.com/spf13/cobra"
)

var (
	displayMode int         = 0
	textLyric   string      = ""
	nowPlaying  string      = ""
	artists     []string    = make([]string, 0)
	picUrl      string      = ""
	picImage    image.Image = nil

	displayModeOne struct {
		label_1 widget.Label
		label_2 widget.Label
		label_3 widget.Label
		label_4 widget.Label
	}

	artistText   *canvas.Text
	songnameText *canvas.Text
	// UI Data Binding
	songNameBinding = binding.NewString()
	artistsBinding  = binding.NewString()
	imageBinding    = binding.NewUntyped()

	fApp fyne.App
	wind fyne.Window
)

var displayCmd = &cobra.Command{
	Use:   "display",
	Short: "Start printing the current lines to stdout",

	RunE: func(cmd *cobra.Command, args []string) error {
		fApp = app.New()
		fApp.Settings().SetTheme(&custom.MyTheme{})
		wind = fApp.NewWindow("KLyrics")
		wind.SetFullScreen(true)

		// initialize UI - its fine to be empty
		artistText = canvas.NewText(strings.Join(artists, ", "), color.White)
		songnameText = canvas.NewText(nowPlaying, color.White)

		songdetailVBox := container.NewVBox(
			artistText,
			songnameText,
		)

		songImage := canvas.NewImageFromImage(nil)
		topsongHBox := container.NewHBox(
			container.NewVBox(layout.NewSpacer()),
			songImage,      // image
			songdetailVBox, // artists + song name
			container.NewVBox(layout.NewSpacer()),
		)

		content := container.NewBorder(topsongHBox, nil, nil, nil, nil)
		wind.SetContent(content)

		wind.SetCloseIntercept(func() {
			fmt.Println("Window closed")
			fApp.Quit()
		})
		go startLyrics()
		wind.ShowAndRun()
		return nil
	},
}

func startLyrics() {
	conf, err := config.Load()
	if err != nil {
		fmt.Println("couldn't load config: ", err)
	}

	if conf == nil {
		conf = config.New()
	}

	if FlagCookie != "" {
		conf.Cookie = FlagCookie
	} else if envCookie := os.Getenv("SPOTIFY_COOKIE"); envCookie != "" {
		conf.Cookie = envCookie
	}

	player, err := config.GetPlayer(conf)
	if err != nil {
		fmt.Println("config getplayer error: ", err)
	}

	var provider lyrics.Provider
	if conf.Cookie != "" {
		if spt, ok := player.(*spotify.Client); ok {
			// use existing spotify client
			provider = spt
		} else {
			// create new client
			provider, _ = spotify.New(conf.Cookie)
		}
	}

	if FlagVerbose {
		conf.IgnoreErrors = false
	}

	var ch = make(chan pool.Update)
	go pool.Listen(player, provider, conf, ch)

	for update := range ch {
		if update.Err != nil {
			if !conf.IgnoreErrors {
				fmt.Println(update.Err.Error())
			}
			continue
		}

		if !reflect.DeepEqual(update.Artists, artists) {
			artists = update.Artists
			artistText.Text = strings.Join(artists, ", ")
			artistText.Refresh()
		}

		if update.NowPlaying != nowPlaying {
			nowPlaying = update.NowPlaying
			songnameText.Text = nowPlaying
			songnameText.Refresh()
			picImage = nil
		}

		if update.PicUrl != picUrl {
			// Download PIC Update image byte
			picUrl = update.PicUrl
			response, err := http.Get(picUrl)
			if err != nil {
				if !conf.IgnoreErrors {
					fmt.Println(err)
				}
			}
			defer response.Body.Close()

			if response.StatusCode != 200 {
				if err != nil {
					if !conf.IgnoreErrors {
						fmt.Println("received non 200 response code")
					}
				}
			}

			img, _, err := image.Decode(response.Body)
			if err != nil {
				if !conf.IgnoreErrors {
					fmt.Println(err)
				}
			}
			newImage := resize.Resize(300, 300, img, resize.Lanczos3)
			picImage = newImage

			// songImage = canvas.NewImageFromImage(picImage)
		}

		if update.Lines == nil || !lyrics.Timesynced(update.Lines) {
			textLyric = nowPlaying
			continue
		}

		line := update.Lines[update.Index].Words
		if conf.Pipe.Length == 0 {
			textLyric = line
		} else {
			switch conf.Pipe.Overflow {
			case "word":
				s := wordwrap.String(line, conf.Pipe.Length)
				textLyric = strings.Split(s, "\n")[0]
			case "none":
				s := wrap.String(line, conf.Pipe.Length)
				textLyric = strings.Split(s, "\n")[0]
			case "ellipsis":
				s := wrap.String(line, conf.Pipe.Length)
				lines := strings.Split(s, "\n")
				if len(lines) == 1 {
					textLyric = lines[0]
				} else {
					s := wrap.String(lines[0], conf.Pipe.Length-3)
					textLyric = strings.Split(s, "\n")[0] + "..."
				}
			}
		}
	}
}
