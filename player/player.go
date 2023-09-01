package player

type Player interface {
	State() (*State, error)
}

type State struct {
	// ID of the current track.
	ID string
	// Query is a string that can be used to find lyrics.
	Query string
	// Position of the current track in ms.
	Position int
	// Playing means whether the track is playing at the moment.
	Playing bool
	// Song name of now playing
	NowPlaying string
	// Artist name of now playing songs
	Artists []string
	PicUrl  string
}
