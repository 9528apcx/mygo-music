package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/music"
)

func (a *app) lyricOffset(tr *music.Track) time.Duration {
	if tr.ShowID != "" {
		return 0
	}
	return time.Duration(a.settings.LyricOffsets[tr.Source+"\x00"+tr.ID]) * time.Millisecond
}

func (a *app) lyricPosition(tr *music.Track) time.Duration {
	return a.pl.Position() + a.lyricOffset(tr)
}

func (a *app) setLyricOffset(tr *music.Track, offset time.Duration) {
	key := tr.Source + "\x00" + tr.ID
	if offset == 0 {
		delete(a.settings.LyricOffsets, key)
	} else {
		if a.settings.LyricOffsets == nil {
			a.settings.LyricOffsets = make(map[string]int64)
		}
		a.settings.LyricOffsets[key] = offset.Milliseconds()
	}
	a.lyricAt = -2
	a.settings.save()
}

func (a *app) lyricTimingControls(c *ui.Context, tr *music.Track) ui.Element {
	offset := a.lyricOffset(tr)
	row := ui.Row(c.Key("lyric-timing")).Gap(2).AlignItems(ui.Center).AlignSelf(ui.End)
	row.Children(func() {
		for _, step := range []struct {
			key, label, hint string
			icon             *ui.SVG
			delta            time.Duration
		}{
			{"earlier", N("Lyrics earlier by 0.25 s"), "- 0.25s", iconLyricsEarlier, 250 * time.Millisecond},
			{"later", N("Lyrics later by 0.25 s"), "+ 0.25s", iconLyricsLater, -250 * time.Millisecond},
		} {
			directionalOffset := offset
			if step.delta < 0 {
				directionalOffset = -offset
			}
			alpha := float32(0.35)
			if directionalOffset > 0 {
				alpha = min(0.8, 0.5+float32(directionalOffset.Seconds())*0.2)
			}
			b := iconButton(c.Key(step.key), step.icon, 20, white.Alpha(alpha), T(step.label)).Tooltip(step.hint)
			if b.Clicked() {
				next := offset + step.delta
				a.setLyricOffset(tr, next)
				switch {
				case next > 0:
					a.toast = Tf("Earlier %.2f s", next.Seconds())
				case next < 0:
					a.toast = Tf("Later %.2f s", -next.Seconds())
				default:
					a.toast = T("Lyrics timing reset")
				}
			}
		}
		if iconButton(c.Key("reset"), iconRefresh, 20, white.Alpha(0.7), T("Reset lyrics timing")).Tooltip(T("Reset lyrics timing")).Disabled(offset == 0).Clicked() {
			a.setLyricOffset(tr, 0)
			a.toast = T("Lyrics timing reset")
		}
	})
	return row
}
