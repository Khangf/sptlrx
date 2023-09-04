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

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
	"github.com/nfnt/resize"
	"github.com/spf13/cobra"
)

var (
	displayMode int         = 0
	nowPlaying  string      = ""
	artists     []string    = make([]string, 0)
	picUrl      string      = ""
	picImage    image.Image = nil

	displayModeOne struct {
		lyricTextLine_1 *canvas.Text
		lyricTextLine_2 *canvas.Text
		lyricTextLine_3 *canvas.Text
		lyricTextLine_4 *canvas.Text
	}

	artistText   *canvas.Text
	songnameText *canvas.Text
	songImage    *canvas.Image

	fApp     fyne.App
	wind     fyne.Window
	windSize fyne.Size

	layoutOne *fyne.Container
)

var displayCmd = &cobra.Command{
	Use:   "display",
	Short: "Start printing the current lines to stdout",

	RunE: func(cmd *cobra.Command, args []string) error {
		fApp = app.New()
		fApp.Settings().SetTheme(&custom.MyTheme{})
		wind = fApp.NewWindow("KLyrics")
		wind.SetFullScreen(true)
		wind.SetCloseIntercept(func() {
			fmt.Println("Window closed")
			fApp.Quit()
		})
		// initialize UI - its fine to be empty
		artistText = canvas.NewText(strings.Join(artists, ", "), color.White)
		artistText.TextSize = 50
		songnameText = canvas.NewText(nowPlaying, color.White)
		songnameText.TextSize = 100
		displayModeOne.lyricTextLine_1 = canvas.NewText("", color.White)
		displayModeOne.lyricTextLine_1.TextSize = 100
		displayModeOne.lyricTextLine_2 = canvas.NewText("", color.White)
		displayModeOne.lyricTextLine_2.TextSize = 100
		displayModeOne.lyricTextLine_3 = canvas.NewText("", color.White)
		displayModeOne.lyricTextLine_3.TextSize = 100
		displayModeOne.lyricTextLine_4 = canvas.NewText("", color.White)
		displayModeOne.lyricTextLine_4.TextSize = 100

		hSpacer := canvas.NewRectangle(color.Transparent)
		hSpacer.SetMinSize(fyne.NewSize(0, 50))
		wSpacer := canvas.NewRectangle(color.Transparent)
		wSpacer.SetMinSize(fyne.NewSize(50, 0))

		songdetailVBox := container.NewVBox(
			hSpacer,
			artistText,
			songnameText,
			hSpacer,
		)

		songImage = canvas.NewImageFromImage(nil)
		songImage.SetMinSize(fyne.NewSize(300, 300))
		// songImage.SetMinSize(fyne.NewSize( 300, 300))
		topsongHBox := container.NewHBox(
			wSpacer,
			container.NewVBox(
				hSpacer,
				songImage, // image
			),
			wSpacer,
			songdetailVBox, // artists + song name
			wSpacer,
		)

		leftLyrics := container.NewVBox(
			hSpacer,
			hSpacer,
			container.NewHBox(
				wSpacer,
				wSpacer,
				wSpacer,
				displayModeOne.lyricTextLine_1,
				wSpacer,
				wSpacer,
				wSpacer,
			),
			container.NewHBox(
				wSpacer,
				wSpacer,
				wSpacer,
				displayModeOne.lyricTextLine_2,
				wSpacer,
				wSpacer,
				wSpacer,
			),
			container.NewHBox(
				wSpacer,
				wSpacer,
				wSpacer,
				displayModeOne.lyricTextLine_3,
				wSpacer,
				wSpacer,
				wSpacer,
			),
			container.NewHBox(
				wSpacer,
				wSpacer,
				wSpacer,
				displayModeOne.lyricTextLine_4,
				wSpacer,
				wSpacer,
				wSpacer,
			),
		)

		layoutOne = container.NewBorder(topsongHBox, nil, leftLyrics, nil, nil)
		wind.SetContent(layoutOne)

		go startLyrics()
		wind.ShowAndRun()
		return nil
	},
}

func startLyrics() {
	currentLyric := ""
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
			songImage.Image = picImage
			songImage.Refresh()
		}

		if update.Lines == nil || !lyrics.Timesynced(update.Lines) {
			currentLyric = nowPlaying
			continue
		}

		line := update.Lines[update.Index].Words
		if conf.Pipe.Length == 0 {
			currentLyric = line
		} else {
			switch conf.Pipe.Overflow {
			case "word":
				s := wordwrap.String(line, conf.Pipe.Length)
				currentLyric = strings.Split(s, "\n")[0]
			case "none":
				s := wrap.String(line, conf.Pipe.Length)
				currentLyric = strings.Split(s, "\n")[0]
			case "ellipsis":
				s := wrap.String(line, conf.Pipe.Length)
				lines := strings.Split(s, "\n")
				if len(lines) == 1 {
					currentLyric = lines[0]
				} else {
					s := wrap.String(lines[0], conf.Pipe.Length-3)
					currentLyric = strings.Split(s, "\n")[0] + "..."
				}
			}
		}

		words := strings.Fields(currentLyric)
		chunks := []string{}
		currentChunk := ""

		// Iterate through the words and add them to the current chunk until it reaches 40 characters
		for _, word := range words {
			if len(currentChunk)+len(word)+1 <= 40 {
				if currentChunk != "" {
					currentChunk += " "
				}
				currentChunk += word
			} else {
				chunks = append(chunks, currentChunk)
				currentChunk = word
			}
		}
		// Add the last chunk
		if currentChunk != "" {
			chunks = append(chunks, currentChunk)
		}

		switch chunk := len(chunks); {
		case chunk >= 4:
			displayModeOne.lyricTextLine_1.Text = chunks[0]
			displayModeOne.lyricTextLine_2.Text = chunks[1]
			displayModeOne.lyricTextLine_3.Text = chunks[2]
			displayModeOne.lyricTextLine_4.Text = chunks[3]
		case chunk == 3:
			displayModeOne.lyricTextLine_1.Text = chunks[0]
			displayModeOne.lyricTextLine_2.Text = chunks[1]
			displayModeOne.lyricTextLine_3.Text = chunks[2]
			displayModeOne.lyricTextLine_4.Text = ""
		case chunk == 2:
			displayModeOne.lyricTextLine_1.Text = chunks[0]
			displayModeOne.lyricTextLine_2.Text = chunks[1]
			displayModeOne.lyricTextLine_3.Text = ""
			displayModeOne.lyricTextLine_4.Text = ""
		default:
			displayModeOne.lyricTextLine_1.Text = chunks[0]
			displayModeOne.lyricTextLine_2.Text = ""
			displayModeOne.lyricTextLine_3.Text = ""
			displayModeOne.lyricTextLine_4.Text = ""
		}
		displayModeOne.lyricTextLine_1.Refresh()
		displayModeOne.lyricTextLine_2.Refresh()
		displayModeOne.lyricTextLine_3.Refresh()
		displayModeOne.lyricTextLine_4.Refresh()
	}
}
