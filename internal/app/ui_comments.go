package app

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/i18n"
	"mygo-music/internal/music"
)

// commentState is the comments of the song or episode Now Playing shows,
// loaded a page at a time as they scroll, for a source that has them.
type commentState struct {
	track   string // whose comments they are: its source and ID
	lib     *library
	id      string // its ID
	list    []music.Comment
	total   int
	more    bool
	loaded  bool
	loading bool
	err     string
	scroll  ui.ScrollState
	commentComposer
	likesPending map[string]bool
}

// commentsFor makes the comments those of tr, loading their first page
// once it changed: Now Playing does it as it shows, for the count on the
// pane's tab.
func (a *app) commentsFor(tr *music.Track) *commentState {
	cs := &a.comments
	if key := tr.Source + "\x00" + tr.ID; cs.track != key {
		*cs = commentState{track: key, lib: a.trackLib(tr), id: tr.ID}
		if cs.lib.can("comments") {
			a.loadComments()
		}
	}
	return cs
}

// loadComments loads the next page of comments.
func (a *app) loadComments() {
	cs := &a.comments
	if cs.loading || cs.lib == nil {
		return
	}
	lib, track, id, offset := cs.lib, cs.track, cs.id, len(cs.list)
	cs.loading, cs.err = true, ""
	go func() {
		page, err := answered(lib.src.Comments(id, offset))
		a.post(func() {
			if cs.track != track {
				return
			}
			cs.loading = false
			if err != nil {
				cs.err = a.problem(lib, err, T("Couldn't load the comments"))
				return
			}
			cs.loaded = true
			// New comments shift the pages: those already shown come again.
			seen := make(map[string]bool, len(cs.list))
			for _, cm := range cs.list {
				seen[cm.ID] = true
			}
			for _, cm := range page.Comments {
				if !seen[cm.ID] {
					cs.list = append(cs.list, cm)
				}
			}
			cs.total, cs.more = page.Total, page.More && len(page.Comments) > 0
			if cs.total < len(cs.list) {
				cs.total = len(cs.list)
			}
		})
	}()
}

// topComments are the comments most liked, for the quiet parts of a song.
func (cs *commentState) topComments() []*music.Comment {
	var top []*music.Comment
	for i := range cs.list {
		if cs.list[i].Likes > 0 {
			top = append(top, &cs.list[i])
		}
	}
	slices.SortStableFunc(top, func(x, y *music.Comment) int { return y.Likes - x.Likes })
	return top[:min(len(top), 10)]
}

// commentCount is the count the comments' tab shows: the last song's
// while the next one's loads, so that it rolls from one to the other.
func (a *app) commentCount() int {
	switch cs := &a.comments; {
	case cs.loaded:
		return cs.total
	case cs.err != "":
		return 0
	}
	return a.tabCount.n
}

// rollState is a count as it rolls from one value to another.
type rollState struct {
	n    int
	up   bool       // whether it went up, last it changed
	exit *ui.Motion // where the characters shown go as it changes
}

// rollLine is how far a character rolls, about a line of the tab's text.
const rollLine = 16

// rollCount shows n as text, each character in a slot of its own, as
// the wheels of a counter: one that changes rolls out as the next rolls
// in, upward as n grows and downward as it shrinks, the others staying.
func rollCount(c *ui.Context, st *rollState, n int, text string, fg ui.Color) {
	if n != st.n {
		st.up = n > st.n
		dir := float32(1)
		if !st.up {
			dir = -1
		}
		if st.exit != nil {
			// The characters shown leave from this frame on, which first
			// knows the way.
			st.exit.Y = -dir * rollLine
		}
		st.n, st.exit = n, &ui.Motion{}
	}
	dir := float32(1)
	if !st.up {
		dir = -1
	}
	chars := []rune(text)
	// Clipped to its line, a character leaves out of sight.
	ui.Row(c).ClipY().Children(func() {
		for i, ch := range chars {
			// Slots go by their place from the right, as the wheels of a
			// counter do: the units are always the last.
			k := len(chars) - 1 - i
			roll := ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: 0.15}, Delay: time.Duration(k) * 30 * time.Millisecond}
			slot := ui.Box(c.Key(k)).Transition(ui.ElementTransition{
				Animation: &roll, Size: true,
				Enter: &ui.Motion{Y: dir * rollLine, Collapse: true},
				Exit:  &ui.Motion{Collapse: true},
			})
			slot.Children(func() {
				ui.Text(c.Key(ch), string(ch)).FontSize(13).FontWeight(600).FontFeatures("tnum").TextColor(fg).
					Transition(ui.ElementTransition{Animation: &roll, Colors: true, Enter: &ui.Motion{Y: dir * rollLine}, Exit: st.exit})
			})
		}
	})
}

// count formats a count shortly, as people read it in the listener's language.
func count(n int) string { return i18n.Count(n) }

// commentsPane lists the comments of tr, which load on as they scroll into
// view, under a place to write one, if the source takes them.
func (a *app) commentsPane(c *ui.Context, tr *music.Track) {
	cs := a.commentsFor(tr)
	_, canReply := cs.lib.src.(music.TrackCommentReplier)
	canReply = canReply && cs.lib.src.Can("replyComment")
	if cs.lib.can("postComment") || canReply && cs.replyID != "" {
		ui.Column(c.Key("composer")).Padding(0, 8, 0, 0).Children(func() {
			commentBox(c, &cs.commentComposer, white, white.Alpha(0.65), white.Alpha(0.14), false, func(text, replyID string) {
				a.postComment(cs, text, replyID)
			})
		})
	}
	if len(cs.list) == 0 {
		if cs.loading || !cs.loaded && cs.err == "" {
			ui.Column(c).Grow(1).Padding(4, 8, 0, 0).Gap(2).Children(func() {
				s := npShimmer(c)
				s.text(c, 13, 1.3).Width(56).Margin(18, 12, 6, 12)
				commentSkeleton(c, s, 6)
			})
			return
		}
		ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
			switch {
			case cs.err != "":
				ui.Text(c, cs.err).FontSize(15).TextColor(white.Alpha(0.5))
				if pillButton(c.Key("retry"), iconComment, T("Retry"), white.Alpha(0.14), white).Clicked() {
					a.loadComments()
				}
			default:
				ui.Icon(c, iconComment).Size(40, 40).TextColor(white.Alpha(0.3))
				ui.Text(c, T("No comments yet")).FontSize(17).FontWeight(600).TextColor(white.Alpha(0.45))
			}
		})
		return
	}
	now := c.Now()
	scroll := ui.Scroll(c.Key("comments")).Grow(1).TrackScroll(&cs.scroll).Padding(4, 8, 48, 0)
	var end ui.Rect
	scroll.Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			for i := range cs.list {
				a.commentRow(c.Key(cs.list[i].ID), &cs.list[i], now, canReply)
			}
			if cs.loading {
				commentSkeleton(c.Key("more"), npShimmer(c), 2)
			}
			ui.Row(c.Key("foot")).Height(56).Center().Children(func() {
				switch {
				case cs.loading:
				case cs.err != "":
					retry := ui.ButtonBase(c.Key("retry")).Padding(6, 14).Radius(999).Background(white.Alpha(0.1)).Label(T("Retry"))
					retry.Children(func() {
						ui.Text(c, T("Couldn't load. Click to try again.")).FontSize(12.5).TextColor(white.Alpha(0.7))
					})
					if retry.Clicked() {
						a.loadComments()
					}
				case !cs.more:
					ui.Text(c, T("That's all the comments")).FontSize(12).TextColor(white.Alpha(0.35))
				}
			})
			end = ui.Box(c.Key("end")).Height(0).Bounds()
		})
	})
	// The next page, once the foot is within a screen of coming into view.
	if box := scroll.Bounds(); cs.more && !cs.loading && cs.err == "" && end.W > 0 && end.Y < box.Y+box.H*2 {
		a.loadComments()
	}
}

// The two tabs share the composer, with a separate draft for the selected reply.
type commentComposer struct {
	draft, replyDraft    string
	replyID, replyAuthor string
	posting, focus       bool
}

func (cs *commentComposer) sent(replyID string) {
	if replyID != "" {
		cs.replyID, cs.replyAuthor, cs.replyDraft = "", "", ""
	} else {
		cs.draft = ""
	}
}

func commentBox(c *ui.Context, cs *commentComposer, fg, muted, surface ui.Color, loading bool, submit func(text, replyID string)) {
	replyID := cs.replyID
	draft, placeholder := &cs.draft, T("Write a comment…")
	if replyID != "" {
		draft, placeholder = &cs.replyDraft, T("Write a reply…")
	}
	ui.Column(c).Gap(4).Padding(8, 0, 12, 0).Children(func() {
		if replyID != "" {
			label := T("Reply")
			if cs.replyAuthor != "" {
				label = Tf("Replying to %s", cs.replyAuthor)
			}
			ui.Text(c, label).FontSize(12).TextColor(muted).SingleLine()
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			in := ui.TextInput(c.Key("draft"), draft).Placeholder(placeholder).Label(placeholder).
				Grow(1).Shrink(1).MinWidth(0).Height(36).Radius(10).BorderWidth(0).Background(surface).
				TextColor(fg).PlaceholderColor(muted).Disabled(cs.posting)
			if cs.focus {
				in.Focus(ui.FocusOptions{PreventScroll: true})
				cs.focus = false
			}
			text := strings.TrimSpace(*draft)
			busy := cs.posting || loading
			send := pillButton(c.Key("send"), nil, T("Post"), surface, fg).Disabled(text == "" || busy)
			if replyID != "" && pillButton(c.Key("cancel"), nil, T("Cancel"), ui.Transparent, muted).Disabled(cs.posting).Clicked() {
				cs.sent(replyID)
				cs.focus = true
				return
			}
			if !busy && text != "" && (send.Clicked() || in.Submitted()) {
				submit(text, replyID)
			}
		})
	})
}

const commentActionSize = 16

func (a *app) commentReplyButton(c *ui.Context, lib *library, cs *commentComposer, cm *music.Comment, color ui.Color) {
	if cm.ID == "" || !iconButton(c, iconComment, commentActionSize, color, T("Reply")).Disabled(cs.posting).Clicked() {
		return
	}
	if lib.signedOut() {
		a.promptSignIn(lib)
		return
	}
	cs.replyID, cs.replyAuthor, cs.focus = cm.ID, cm.Author, true
}

// postComment posts what the listener wrote, showing it first of the comments
// once the source took it.
func (a *app) postComment(cs *commentState, text, replyID string) {
	lib, track, id := cs.lib, cs.track, cs.id
	if lib.signedOut() {
		a.promptSignIn(lib)
		return
	}
	cs.posting = true
	go func() {
		var cm *music.Comment
		var err error
		if replyID != "" {
			cm, err = lib.src.(music.TrackCommentReplier).ReplyComment(id, replyID, text)
		} else {
			cm, err = lib.src.PostComment(id, text)
		}
		a.post(func() {
			if cs.track != track {
				return
			}
			cs.posting = false
			if err != nil {
				a.actionFailed(lib, err, T("Couldn't post your comment. Try again later."))
				return
			}
			cs.sent(replyID)
			if cm != nil {
				cs.list = append([]music.Comment{*cm}, cs.list...)
				cs.total++
			}
		})
	}()
}

// commentRow is a comment: who wrote it, what, when, and how many liked it,
// with a thumb to like it too.
func (a *app) commentRow(c *ui.Context, cm *music.Comment, now time.Time, canReply bool) {
	ui.Row(c).Padding(12).Gap(12).Radius(12).AlignItems(ui.Start).Children(func() {
		a.avatar(c, cm.AvatarURL, 34, white.Alpha(0.1))
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, cm.Author).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.6)).SingleLine().Shrink(1).MinWidth(0)
			ui.Text(c, cm.Text).FontSize(15).LineHeight(1.55).TextColor(white.Alpha(0.92))
			ui.Row(c.Key("comment-meta")).Gap(6).AlignItems(ui.Center).Children(func() {
				if cm.Time > 0 {
					ui.Text(c, i18n.Ago(time.UnixMilli(cm.Time), now)).FontSize(11.5).TextColor(white.Alpha(0.38))
				}
				cs := &a.comments
				lib, track, trackID, id := cs.lib, cs.track, cs.id, cm.ID
				color := white.Alpha(0.65)
				ui.Spacer(c)
				if a.commentLike(c.Key("like"), cm, color, lib.can("likeComment"), cs.likesPending[id]) {
					a.toggleCommentLike(lib, cm, &cs.likesPending, func() *music.Comment {
						if cs.track != track {
							return nil
						}
						return findComment(cs.list, id)
					}, func(on bool) error { return lib.src.LikeComment(trackID, id, on) })
				}
				if canReply {
					a.commentReplyButton(c.Key("reply"), lib, &cs.commentComposer, cm, color)
				}
			})
		})
	})
}

// commentLike uses the same borderless target and icon size as Reply.
func (a *app) commentLike(c *ui.Context, cm *music.Comment, fg ui.Color, can, pending bool) bool {
	can = can && cm.ID != ""
	if !can && cm.Likes == 0 {
		return false
	}
	label, icon := T("Like"), iconThumb
	if cm.Liked {
		label, icon, fg = T("Unlike"), iconThumbFill, a.pal.accent
	}
	b := roundButton(c, commentActionSize, fg, label).Disabled(!can || pending).Gap(4)
	var n string
	if cm.Likes > 0 {
		n = count(cm.Likes)
		w, _ := c.MeasureText(0, ui.Span{Text: n, Size: 12})
		b.Width(commentActionSize + 20 + w)
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(commentActionSize, commentActionSize)
		if n != "" {
			ui.Text(c, n).FontSize(12).FontFeatures("tnum")
		}
	})
	return b.Clicked()
}

// Keep each comment's likes in order and bring back the old state if it fails.
func (a *app) toggleCommentLike(lib *library, cm *music.Comment, pending *map[string]bool, find func() *music.Comment, request func(bool) error) {
	if lib.signedOut() {
		a.promptSignIn(lib)
		return
	}
	if *pending == nil {
		*pending = make(map[string]bool)
	}
	queue, id := *pending, cm.ID
	if queue[id] {
		return
	}
	queue[id] = true
	was := cm.Liked
	setCommentLiked(cm, !was)
	go func() {
		err := request(!was)
		a.post(func() {
			delete(queue, id)
			if err != nil {
				if current := find(); current != nil {
					setCommentLiked(current, was)
					a.actionFailed(lib, err, T("That didn't work. Try again later."))
				}
			}
		})
	}()
}

func findComment(list []music.Comment, id string) *music.Comment {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

func setCommentLiked(cm *music.Comment, on bool) {
	if cm.Liked == on {
		return
	}
	cm.Liked = on
	if on {
		cm.Likes++
	} else {
		cm.Likes = max(cm.Likes-1, 0)
	}
}
