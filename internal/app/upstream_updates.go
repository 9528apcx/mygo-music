package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/egoist/mygo/ui"
)

const customNetEaseRepo = "9528apcx/mygo-music-netease-cloud-music"

var upstreamSources = [...]struct{ repo, label string }{
	{"egoist/mygo-music", N("Player upstream")},
	{"AkinoKaede/mygo-music-netease-cloud-music", N("NetEase plugin upstream")},
}

func upstreamCommits(client *http.Client, repo string) (int, error) {
	url := "https://api.github.com/repos/" + repo + "/compare/9528apcx:mygo-custom...main"
	response, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GitHub: %s", response.Status)
	}
	var comparison struct {
		Ahead *int `json:"ahead_by"`
	}
	if err := json.NewDecoder(response.Body).Decode(&comparison); err != nil {
		return 0, err
	}
	if comparison.Ahead == nil {
		return 0, fmt.Errorf("GitHub comparison has no ahead count")
	}
	return *comparison.Ahead, nil
}

func (a *app) upstreamUpdateRows(c *ui.Context, row settingsRow, p palette) {
	row(T("Upstream Updates"), T("Check for upstream changes that have not reached the custom branch"), func() {
		if ui.Button(c.Key("check-upstream"), T("Check Upstream")).Disabled(a.checkingUpstream).Clicked() {
			a.checkingUpstream = true
			go func() {
				client := &http.Client{Timeout: 12 * time.Second}
				var status [len(upstreamSources)]string
				for i, source := range upstreamSources {
					ahead, err := upstreamCommits(client, source.repo)
					switch {
					case err != nil:
						status[i] = T("Unable to check GitHub")
					case ahead > 0:
						status[i] = Tf("%d upstream commits to merge", ahead)
					default:
						status[i] = T("Upstream is synced")
					}
				}
				a.post(func() {
					a.upstreamStatus = status
					a.checkingUpstream = false
				})
			}()
		}
	})
	for i, source := range upstreamSources {
		if status := a.upstreamStatus[i]; status != "" {
			row(T(source.label), status, func() {})
		}
	}
	for _, lib := range a.allLibs {
		if lib.id != "netease-cloud-music" {
			continue
		}
		plug := pluginOf(lib)
		if plug == nil || plug.Origin() == nil || plug.Origin().Repo != customNetEaseRepo {
			continue
		}
		row(T("Custom NetEase Plugin"), "", func() {
			a.pluginUpdateError(c, lib, p)
			a.pluginUpdateButton(c, lib)
			if ui.Button(c.Key("check-custom-netease"), T("Check for Updates")).Disabled(a.installer.checked[lib.id] == checking).Clicked() {
				a.checkPluginUpdate(lib)
			}
		})
	}
}
