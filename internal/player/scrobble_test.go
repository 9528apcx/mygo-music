package player

import (
	"testing"
	"time"

	"mygo-music/internal/music"
)

type listeningSource struct {
	nothing
	heard chan time.Duration
}
type listeningStream struct{ silence }

func (listeningStream) Close() error                              { return nil }
func (listeningStream) source() (*download, *spectrum)            { return nil, nil }
func (s listeningSource) Scrobble(_ music.Track, d time.Duration) { s.heard <- d }

// Seeking must not count skipped audio, and stopping after an end must not submit twice.
func TestScrobbleCountsAudioOnce(t *testing.T) {
	s := listeningSource{heard: make(chan time.Duration, 2)}
	p := &Player{track: &music.Track{ID: "1", Source: "s"}, Source: func(string) music.Source { return s }}
	w := &watched{stream: listeningStream{}, now: time.Now}
	p.watch = w
	b := make([]float32, 48000*2)
	w.Read(b)
	w.SetPosition(48000 * 200)
	w.Read(b)
	p.scrobble()
	if w.samples.Load() != 0 {
		t.Fatal("reported audio was not cleared")
	}
	p.scrobble()
	select {
	case got := <-s.heard:
		if got != 2*time.Second {
			t.Fatalf("listened %v, want 2s", got)
		}
	case <-time.After(time.Second):
		t.Fatal("no listening report")
	}
	select {
	case <-s.heard:
		t.Fatal("duplicate report")
	default:
	}
}
