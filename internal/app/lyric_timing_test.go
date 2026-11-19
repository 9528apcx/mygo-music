package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/i18n"
	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
)

// Timing buttons change lyrics in both views without seeking audio, and persist per song.
func TestLyricTimingControls(t *testing.T) {
	for _, pane := range []int{paneLyrics, paneStage} {
		a, tt, _ := playingApp()
		a.settings.path = filepath.Join(t.TempDir(), "settings.json")
		a.nowPlaying, a.npPane = true, pane
		a.pl.Seek(9750 * time.Millisecond)
		tt.Move(600, 400)
		tt.Advance(time.Second)
		tr := a.pl.Current()
		for _, step := range []struct {
			label string
			want  time.Duration
		}{
			{"Lyrics earlier by 0.25 s", 250 * time.Millisecond},
			{"Lyrics earlier by 0.25 s", 500 * time.Millisecond},
			{"Lyrics later by 0.25 s", 250 * time.Millisecond},
		} {
			if err := tt.Click(step.label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := a.lyricOffset(tr); got != step.want {
				t.Fatalf("offset %v, want %v", got, step.want)
			}
			if a.pl.Position() != 9750*time.Millisecond {
				t.Fatal("timing adjustment sought audio")
			}
		}
		lines, _ := a.pl.Lyrics()
		if lyrics.Index(lines, a.lyricPosition(tr)) != 1 {
			t.Fatal("offset did not advance the active line")
		}
		b, err := os.ReadFile(a.settings.path)
		if err != nil {
			t.Fatal(err)
		}
		var stored settings
		if err = json.Unmarshal(b, &stored); err != nil {
			t.Fatal(err)
		}
		restored := &app{settings: stored}
		if restored.lyricOffset(tr) != 250*time.Millisecond {
			t.Fatal("timing was not persisted")
		}
		other := *tr
		other.Source = "another-source"
		if a.lyricOffset(&other) != 0 {
			t.Fatal("offset leaked across sources")
		}
		other = *tr
		other.ID = "another-song"
		if a.lyricOffset(&other) != 0 {
			t.Fatal("offset leaked across tracks")
		}
		if err := tt.Click("Reset lyrics timing"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.lyricOffset(tr) != 0 || len(a.settings.LyricOffsets) != 0 {
			t.Fatal("reset did not remove song timing")
		}
		if err := tt.Click("Lyrics later by 0.25 s"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.lyricOffset(tr) != -250*time.Millisecond || a.lyricPosition(tr) != 9500*time.Millisecond {
			t.Fatal("delaying lyrics did not move their clock back")
		}
	}
}

// With MM_FRAMES set, saves Chinese timing controls in normal and narrow lyrics views.
func TestLyricTimingFrames(t *testing.T) {
	dir := framesDir(t)
	i18n.SetLanguage(i18n.Chinese)
	defer i18n.SetLanguage(i18n.English)
	for _, size := range [][2]int{{1180, 780}, {600, 1000}} {
		for _, pane := range []int{paneLyrics, paneStage} {
			a, _, _ := playingApp()
			a.nowPlaying, a.npPane = true, pane
			a.setLyricOffset(a.pl.Current(), -1500*time.Millisecond)
			a.pl.Hold(12 * time.Second)
			tt := ui.NewTester(a.view, size[0], size[1])
			tt.Move(300, 300)
			settled(tt)
			saveFrame(t, dir, fmt.Sprintf("lyric-sync-%d-%d.png", pane, size[0]), tt.Image())
		}
	}
}

// Clicking a line after correction seeks to the matching audio time, clamped at the start.
func TestLyricTimingSeek(t *testing.T) {
	a := newTestApp()
	tr := music.Track{ID: "timed", Source: "fake", Duration: 60000}
	a.pl.Cue([]music.Track{tr})
	a.pl.SetLyrics(lyrics.Build("[00:00.20]First line\n[00:10.00]Second line", ""))
	a.pl.Hold(time.Second)
	a.setLyricOffset(&tr, 500*time.Millisecond)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1200, 800)
	settled(tt)
	if err := tt.Click("Second line"); err != nil {
		t.Fatal(err)
	}
	if got := a.pl.Position(); got != 9500*time.Millisecond {
		t.Fatalf("seek %v, want 9.5s", got)
	}
	a.pl.Hold(time.Second)
	settled(tt)
	if err := tt.Click("First line"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Position() != 0 {
		t.Fatal("seek went before start")
	}
}
