package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/i18n"
	"mygo-music/internal/music"
)

// albumData is what the app has of an album.
type albumData struct {
	tab             int
	album           *music.AlbumPage
	loading         bool
	saving          bool
	err             string
	comments        []music.Comment
	commentsPaged   paged
	descriptionOpen bool
	commentDraft    string
	replyDraft      string
	replyTo         string
	posting         bool
}

const (
	albumSongs = iota
	albumComments
	albumDetails
)

func (a *app) loadAlbum(lib *library, id string) {
	d := lib.albums[id]
	if d == nil {
		d = &albumData{}
		lib.albums[id] = d
	}
	if d.album != nil || d.loading {
		return
	}
	d.loading, d.err = true, ""
	go func() {
		al, err := answered(lib.src.Album(id))
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err = a.problem(lib, err, T("Couldn't load this album"))
				return
			}
			d.album = al
		})
	}()
}

func (a *app) albumPage(c *ui.Context) {
	id := a.route.id
	lib := a.library
	d := a.albums[id]
	if d == nil {
		a.loadAlbum(lib, id)
		d = a.albums[id]
	}
	if d.album == nil {
		if !d.loading {
			a.centered(c, d.err, func() { a.loadAlbum(lib, id) })
			return
		}
		s := a.pageShimmer(c)
		a.trackSkeleton(c, s, false, func(c *ui.Context) { a.playlistHeaderSkeleton(c, s) })
		return
	}
	source, _ := lib.src.(music.AlbumCommentSource)
	if !d.album.HasComments {
		source = nil
	}
	header := func(c *ui.Context) {
		a.albumHeader(c, d)
		a.albumTabs(c.Key("tabs"), d, source != nil)
	}
	switch d.tab {
	case albumComments:
		a.scrollPage(c, func() {
			header(c)
			a.albumCommentList(c.Key("comment-list"), lib, d, id, source)
		})
	case albumDetails:
		a.scrollPage(c, func() {
			header(c)
			if text := strings.TrimSpace(d.album.Description); text != "" {
				a.albumDescription(c.Key("description"), d, text)
			} else {
				a.empty(c, T("No album description"))
			}
		})
	default:
		a.trackRows(c, d.album.Tracks, false, header)
	}
}

func (a *app) albumTabs(c *ui.Context, d *albumData, hasComments bool) {
	p := a.pal
	songs := len(d.album.Tracks)
	tabs := []struct {
		label string
		total *int
	}{
		{T("Songs"), &songs}, {T("Comments"), d.album.CommentCount}, {T("Album Details"), nil},
	}
	ui.Row(c).Role(ui.RoleTabList).Gap(14).Padding(0, 0, 14, 0).Children(func() {
		for tab, item := range tabs {
			if tab == albumComments && !hasComments {
				continue
			}
			selected := d.tab == tab
			fg, weight := p.muted, 400
			if selected {
				fg, weight = p.text, 700
			}
			button := ui.ButtonBase(c.Key(tab)).Role(ui.RoleTab).Label(item.label).AccessSelected(selected).Column().Gap(5).Padding(5, 8, 0, 8).AlignItems(ui.Center)
			if button.Hovered() {
				fg = p.text
			}
			button.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Start).Children(func() {
					ui.Text(c, item.label).FontSize(15).FontWeight(weight).TextColor(fg)
					if item.total != nil {
						ui.Text(c, strconv.Itoa(*item.total)).FontSize(11).TextColor(fg)
					}
				})
				line := ui.Box(c).Size(16, 2).Radius(1)
				if selected {
					line.Background(p.accent)
				}
			})
			if button.Clicked() && !selected {
				d.tab = tab
				a.pageScroll.ScrollTo(0, 0, ui.Animation{})
				a.trackList.ScrollTo(0, ui.Start)
			}
		}
	})
}

func (a *app) albumDescription(c *ui.Context, d *albumData, text string) {
	const font = 14
	width, _ := c.Size()
	// Match the track list's content width, including its sidebar and insets.
	width = max(1, width-sidebarWidth-2*(pagePad-12))
	_, full := c.MeasureText(width, ui.Span{Text: text, Size: font})
	_, preview := c.MeasureText(width, ui.Span{Text: "M\nM\nM\nM", Size: font})
	_, fiveLines := c.MeasureText(width, ui.Span{Text: "M\nM\nM\nM\nM", Size: font})
	section := ui.Column(c).Padding(0, 0, 18, 0)
	section.Children(func() {
		// A fifth line alone does not justify hiding content behind a button.
		if full <= fiveLines+0.5 {
			ui.Text(c, text).Width(width).FontSize(font).TextColor(a.pal.muted)
			return
		}

		top, _ := a.insets(c)
		// Keep the description below the toolbar when it is scrolled to.
		anchor := ui.Box(c.Key("anchor")).Height(top+pageGap).Margin(-top-pageGap, 0, 0, 0)
		motion := ui.Animation{Duration: 300 * time.Millisecond, Ease: ui.EaseInOut}
		height, label, icon := preview, T("Show More"), iconDown
		if d.descriptionOpen {
			height, label, icon = full, T("Show Less"), iconUp
		}
		body := ui.Column(c.Key("body")).ClipY()
		body.Height(body.AnimateTo("description-height", height, motion))
		if !d.descriptionOpen {
			opaque := ui.RGB(0, 0, 0)
			body.MaskLinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
				{Color: opaque, At: 0},
				{Color: opaque, At: 0.73},
				{Color: ui.Transparent, At: 1},
			}})
		}
		body.Children(func() {
			ui.Text(c, text).Width(width).FontSize(font).TextColor(a.pal.muted).Shrink(0)
		})
		button := pillButton(c.Key("toggle"), icon, label, a.pal.hover.Over(a.pal.bg), a.pal.text)
		if button.AlignSelf(ui.Center).Margin(8, 0, 0, 0).MinHeight(40).Clicked() {
			if d.descriptionOpen && section.Bounds().Y < top {
				anchor.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollStart, Animation: &motion})
			}
			d.descriptionOpen = !d.descriptionOpen
		}
	})
}

func (a *app) albumHeader(c *ui.Context, d *albumData) {
	p := a.pal
	al := &d.album.Album
	tracks := d.album.Tracks
	ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
		ui.Box(c).Size(212, 212).Radius(14).Shadow(0, 18, 40, -10, p.cardShade).Children(func() {
			a.coverFill(c, al.ImageURL, 600, 14)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, T("Album")).FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, al.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
			ui.RichText(c).FontSize(13).TextColor(p.muted).Children(func() {
				for i, ar := range al.Artists {
					if i > 0 {
						ui.Text(c, ", ")
					}
					if ar.ID != "" && a.can("artist") {
						a.link(c.Key(i), ar.Name, p.muted, p.text, route{page: pageArtist, id: ar.ID})
					} else {
						ui.Text(c, ar.Name)
					}
				}
				meta := []string{i18n.Songs(len(tracks))}
				if al.Released != "" {
					meta = append([]string{al.Released}, meta...)
				}
				sep := ""
				if len(al.Artists) > 0 {
					sep = " · "
				}
				ui.Text(c, sep+strings.Join(meta, " · "))
			})
			ui.Row(c).Gap(10).Margin(10, 0, 0, 0).Children(func() {
				a.playButtons(c, tracks)
				if a.can("saveAlbum") {
					icon, label := iconPlus, TC("Save", "library")
					if al.Saved {
						icon, label = iconCheck, T("Saved")
					}
					if pillButton(c.Key("save"), icon, label, p.hover.Over(p.bg), p.text).Clicked() {
						a.toggleAlbumSaved(a.library, d)
					}
				}
			})
		})
	})
}

func (a *app) albumCommentList(c *ui.Context, lib *library, d *albumData, id string, source music.AlbumCommentSource) {
	pg := &d.commentsPaged
	next := func() {
		pg.loading, pg.err = true, ""
		offset := len(d.comments)
		go func() {
			page, err := answered(source.AlbumComments(id, offset))
			a.post(func() {
				pg.loading = false
				if err != nil {
					pg.err = a.problem(lib, err, T("Couldn't load comments"))
					return
				}
				d.comments = append(d.comments, page.Comments...)
				if page.Total > 0 || len(d.comments) == 0 {
					d.album.CommentCount = &page.Total
				}
				pg.loaded, pg.more = true, page.More && len(page.Comments) > 0
			})
		}()
	}
	if !pg.loaded && !pg.loading && pg.err == "" {
		next()
	}
	writer, canPost := source.(music.AlbumCommentWriter)
	canPost = canPost && lib.src.Can("postAlbumComment")
	if canPost && d.replyTo == "" {
		a.albumCommentBox(c.Key("post"), lib, d, writer, id, "")
	}
	for i, cm := range d.comments {
		a.albumCommentRow(c.Key(i), lib, d, id, cm, writer, canPost)
	}
	switch {
	case pg.loading:
		a.empty(c, T("Loading…"))
	case pg.err != "":
		ui.Text(c, pg.err).TextColor(a.pal.muted)
		if ui.Button(c.Key("retry"), T("Retry")).Disabled(d.posting).Clicked() {
			next()
		}
	case pg.more:
		if ui.Button(c.Key("more"), T("Load More")).Disabled(d.posting).Clicked() {
			next()
		}
	case pg.loaded && len(d.comments) == 0:
		a.empty(c, T("No comments yet"))
	}
}

func (a *app) albumCommentRow(c *ui.Context, lib *library, d *albumData, id string, cm music.Comment, writer music.AlbumCommentWriter, canPost bool) {
	p := a.pal
	ui.Row(c).Gap(12).Padding(12, 0).AlignItems(ui.Start).Children(func() {
		a.avatar(c.Key("avatar"), cm.AvatarURL, 36, p.hover.Over(p.bg))
		ui.Column(c.Key("body")).Grow(1).Shrink(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, cm.Author).FontSize(14).FontWeight(600).TextColor(p.text)
			ui.Text(c, cm.Text).FontSize(15).LineHeight(1.5).TextColor(p.text)
			ui.Row(c.Key("meta")).Gap(12).AlignItems(ui.Center).Children(func() {
				if cm.AuthorID != "" {
					ui.Text(c, "ID: "+cm.AuthorID).FontSize(12).TextColor(p.muted)
				}
				if cm.Time > 0 {
					ui.Text(c, i18n.Date(time.UnixMilli(cm.Time).Local())).FontSize(12).TextColor(p.muted)
				}
				if canPost && cm.ID != "" && d.replyTo == "" && ui.Button(c.Key("reply"), T("Reply")).Disabled(d.posting).Clicked() {
					if lib.signedOut() {
						a.promptSignIn(lib)
					} else {
						d.replyTo = cm.ID
					}
				}
			})
			if canPost && cm.ID != "" && d.replyTo == cm.ID {
				a.albumCommentBox(c.Key("reply-box"), lib, d, writer, id, cm.ID)
			}
		})
	})
}

func (a *app) albumCommentBox(c *ui.Context, lib *library, d *albumData, writer music.AlbumCommentWriter, id, replyTo string) {
	draft, placeholder := &d.commentDraft, T("Write a comment…")
	if replyTo != "" {
		draft, placeholder = &d.replyDraft, T("Write a reply…")
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 0).Children(func() {
		in := ui.TextInput(c.Key("draft"), draft).Placeholder(placeholder).Label(placeholder).Grow(1).Shrink(1).MinWidth(0).Disabled(d.posting)
		text := strings.TrimSpace(*draft)
		busy := d.posting || d.commentsPaged.loading
		send := ui.Button(c.Key("send"), T("Post")).Disabled(text == "" || busy)
		if replyTo != "" && ui.Button(c.Key("cancel"), T("Cancel")).Disabled(d.posting).Clicked() {
			d.replyTo, d.replyDraft = "", ""
			return
		}
		if !(send.Clicked() || in.Submitted()) || text == "" || busy {
			return
		}
		if lib.signedOut() {
			a.promptSignIn(lib)
			return
		}
		d.posting = true
		go func() {
			_, err := writer.PostAlbumComment(id, text, replyTo)
			a.post(func() {
				d.posting = false
				if err != nil {
					a.actionFailed(lib, err, T("Couldn't post your comment. Try again later."))
					return
				}
				*draft, d.replyTo = "", ""
				d.comments, d.commentsPaged = nil, paged{}
			})
		}()
	})
}

// toggleAlbumSaved saves an album or lets it go, at once in the app and then
// with its source, going back if it refuses.
func (a *app) toggleAlbumSaved(lib *library, d *albumData) {
	if lib.signedOut() {
		a.promptSignIn(lib)
		return
	}
	if d.saving {
		return
	}
	al := &d.album.Album
	save := !al.Saved
	al.Saved, d.saving = save, true
	id := al.ID
	go func() {
		err := lib.src.SaveAlbum(id, save)
		a.post(func() {
			d.saving = false
			if err != nil {
				al.Saved = !save
				a.actionFailed(lib, err, T("That didn't work. Try again later."))
				return
			}
			// Their page lists it, or doesn't, once it loads anew.
			lib.savedAlbums, lib.savedAlbumsPaged = nil, paged{}
			if a.route.page == pageSavedAlbums && a.route.src == lib.id {
				a.loadSavedAlbums(lib)
			}
		})
	}()
}

// loadSavedAlbums loads the next page of the albums the listener saved.
func (a *app) loadSavedAlbums(lib *library) {
	if lib.savedAlbumsPaged.loading {
		return
	}
	lib.savedAlbumsPaged.loading, lib.savedAlbumsPaged.err = true, ""
	offset := len(lib.savedAlbums)
	go func() {
		page, err := answered(lib.src.SavedAlbums(offset))
		a.post(func() {
			lib.savedAlbumsPaged.loading = false
			if err != nil {
				lib.savedAlbumsPaged.err = a.problem(lib, err, T("Couldn't load your albums"))
				return
			}
			lib.savedAlbums = append(lib.savedAlbums, page.Albums...)
			lib.savedAlbumsPaged.loaded, lib.savedAlbumsPaged.more = true, page.More && len(page.Albums) > 0
		})
	}()
}

// savedAlbumsPage lists the albums the listener saved.
func (a *app) savedAlbumsPage(c *ui.Context) {
	lib := a.library
	a.scrollPage(c, func() {
		ui.Box(c).PaddingX(12).Children(func() { a.heading(c, a.builtinTitle("savedAlbums"), "") })
		ui.Box(c).PaddingX(12).Children(func() { a.albumGrid(c, lib.savedAlbums) })
		a.pageFoot(c, &lib.savedAlbumsPaged, func() { a.loadSavedAlbums(lib) })
		if lib.savedAlbumsPaged.loaded && len(lib.savedAlbums) == 0 {
			a.empty(c, T("Albums you save will show here"))
		}
	})
}

// albumGrid shows albums as cards, each opening its page.
func (a *app) albumGrid(c *ui.Context, albums []music.Album) {
	ui.Grid(c).Columns(a.columns(c, 156)).GapX(20).GapY(28).Children(func() {
		for i := range albums {
			a.albumCard(c.Key(albums[i].ID), &albums[i])
		}
	})
}

// artistNames joins the names of an album's artists.
func artistNames(artists []music.Artist) string {
	names := make([]string, len(artists))
	for i, ar := range artists {
		names[i] = ar.Name
	}
	return strings.Join(names, ", ")
}

// albumCard is an album's cover, lifting under the pointer, over its name
// and the year it came out.
func (a *app) albumCard(c *ui.Context, al *music.Album) {
	p := a.pal
	card := ui.ButtonBase(c).Column().Gap(10).AlignItems(ui.Stretch).FocusRing(false).Label(al.Name)
	hovered := card.Hovered()
	card.Children(func() {
		art := ui.Box(c).AspectRatio(1).Radius(10)
		lift := ui.Pose{}
		if hovered {
			lift = ui.Pose{Y: -3}
			art.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			art.Shadow(0, 2, 8, -2, p.cardShade)
		}
		art.AnimatePose(lift, ui.Snappy)
		art.Children(func() { a.coverFill(c, al.ImageURL, 400, 10) })
		ui.Text(c, al.Name).FontSize(13).FontWeight(500).MaxLines(2).LineHeight(1.35)
		var meta []string
		if a.route.page != pageArtist && len(al.Artists) > 0 {
			meta = append(meta, artistNames(al.Artists))
		}
		if len(al.Released) >= 4 {
			meta = append(meta, al.Released[:4])
		}
		ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(p.muted).SingleLine().Margin(-6, 0, 0, 0)
	})
	if card.Clicked() && a.can("album") {
		a.open(route{page: pageAlbum, id: al.ID})
	}
}
