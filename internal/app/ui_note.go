package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/music"
)

type noteDialog struct {
	lib                  *library
	writer               music.NoteSharer
	kind, id, name, text string
	open, busy           bool
	err                  string
}

func noteWriter(lib *library) music.NoteSharer {
	writer, ok := lib.src.(music.NoteSharer)
	if ok && lib.src.Can("shareNote") {
		return writer
	}
	return nil
}

func (a *app) openMusicNote(lib *library, writer music.NoteSharer, kind, id, name string) {
	if a.note.busy {
		return
	}
	if lib.signedOut() {
		a.promptSignIn(lib)
		return
	}
	a.note = noteDialog{lib: lib, writer: writer, kind: kind, id: id, name: name, open: true}
}

func (a *app) noteShareDialog(c *ui.Context) {
	p, d := a.pal, &a.note
	ui.DialogBase(c.Key("music-note"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(430).Padding(24).Radius(18).Gap(12).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.4))
		ui.Text(c, T("Share to Music Notes")).FontSize(18).FontWeight(700)
		ui.Text(c, d.name).FontSize(15).FontWeight(600).MaxLines(2)
		hint := T("Write a note…")
		ui.TextArea(c.Key("text"), &d.text).Placeholder(hint).Label(hint).Lines(3, 6).
			AutoFocus().Radius(10).BorderWidth(0).Background(p.hover.Over(p.bg)).Disabled(d.busy)
		ui.Text(c, T("This will publish to NetEase Music Notes.")).FontSize(12).TextColor(p.muted)
		if d.err != "" {
			ui.Text(c, d.err).FontSize(13).TextColor(p.text)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if pillButton(c.Key("cancel"), nil, T("Cancel"), ui.Transparent, p.text).Disabled(d.busy).Clicked() {
				d.open = false
			}
			label := T("Publish Note")
			if d.busy {
				label = T("Publishing Note…")
			}
			if pillButton(c.Key("publish"), nil, label, p.accent, p.onAccent).Disabled(d.busy).Clicked() {
				a.publishMusicNote()
			}
		})
	})
}

func (a *app) publishMusicNote() {
	d := &a.note
	d.busy, d.err = true, ""
	text := strings.TrimSpace(d.text)
	go func() {
		err := d.writer.ShareNote(d.kind, d.id, text)
		a.post(func() {
			d.busy = false
			if err != nil {
				d.err = a.problem(d.lib, err, T("Couldn't confirm publication. Check Music Notes before retrying."))
				return
			}
			a.note = noteDialog{}
			a.toast = T("Note published")
		})
	}()
}
