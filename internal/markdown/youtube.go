package markdown

import (
	"html/template"
	"net/url"
	"regexp"
	"strings"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// 유튜브 링크는 카드 대신 **볼 수 있는 자리**로 그린다.
//
// **누르기 전에는 유튜브에 아무것도 요청하지 않는다.** 섬네일조차 받지 않는다 —
// 그것도 유튜브 서버에서 오는 것이라, 글을 열기만 해도 독자 IP가 제3자에게
// 간다. 파비콘을 외부에서 불러오지 않기로 한 것과 같은 이유다.
// 눌렀을 때 비로소 `youtube-nocookie.com` 플레이어를 끼운다
// (`internal/web/static/youtube.js`).
//
// 스크립트가 못 뜨면 그냥 유튜브로 가는 링크다. 빈 자리가 되지 않는다.

// ytVideoID는 주소에서 영상 id를 뽑는다. 없으면 빈 문자열이다.
//
// watch?v=, youtu.be/, embed/, shorts/ 네 형태를 본다. 재생목록(list)은
// 따로 들고 있다가 플레이어에 같이 넘긴다 — 원래 링크가 목록의 한 편을
// 가리키고 있었으면 그 맥락을 잃지 않는다.
func ytVideoID(u *url.URL) (id, list string) {
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	q := u.Query()
	list = q.Get("list")

	switch host {
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com", "m.youtube.com", "youtube-nocookie.com":
		switch {
		case u.Path == "/watch":
			id = q.Get("v")
		case strings.HasPrefix(u.Path, "/embed/"):
			id = strings.TrimPrefix(u.Path, "/embed/")
		case strings.HasPrefix(u.Path, "/shorts/"):
			id = strings.TrimPrefix(u.Path, "/shorts/")
		}
	}
	if !ytIDPattern.MatchString(id) {
		return "", ""
	}
	if !ytIDPattern.MatchString(list) {
		list = ""
	}
	return id, list
}

// ytIDPattern은 영상·재생목록 id에 쓰이는 글자만 받는다. 주소가 그대로 HTML
// 속성으로 나가므로 형태를 좁혀둔다.
var ytIDPattern = regexp.MustCompile(`^[\w-]{6,64}$`)

// renderYouTube는 유튜브 링크를 "누르면 켜지는 재생 자리"로 그린다.
//
// 겉은 여전히 <a href="원래 주소">다. 스크립트가 못 뜨면 그대로 유튜브로 가고,
// 뜨면 그 자리에서 플레이어로 바뀐다(static/youtube.js). 눌러야 요청이 나가므로
// 글을 열기만 해서는 유튜브에 아무것도 가지 않는다.
func renderYouTube(w util.BufWriter, n *extCardNode) (gast.WalkStatus, error) {
	write := func(parts ...string) error {
		for _, p := range parts {
			if _, err := w.WriteString(p); err != nil {
				return err
			}
		}
		return nil
	}
	if err := write(`<a class="ytembed" rel="noreferrer" href="`); err != nil {
		return gast.WalkStop, err
	}
	template.HTMLEscape(w, []byte(n.URL))
	if err := write(`" data-yt="`, n.YTVideo, `"`); err != nil {
		return gast.WalkStop, err
	}
	if n.YTList != "" {
		if err := write(` data-yt-list="`, n.YTList, `"`); err != nil {
			return gast.WalkStop, err
		}
	}
	// 재생 삼각형은 우리가 그린다. 섬네일을 받아오면 그 순간 유튜브에 요청이 간다.
	if err := write(`><span class="ytembed-play" aria-hidden="true">`,
		`<svg viewBox="0 0 68 48"><path class="ytembed-body" d="M66.5 7.7a8.6 8.6 0 0 0-6-6C55.2 0 34 0 34 0S12.8 0 7.5 1.7a8.6 8.6 0 0 0-6 6A90 90 0 0 0 0 24a90 90 0 0 0 1.5 16.3 8.6 8.6 0 0 0 6 6C12.8 48 34 48 34 48s21.2 0 26.5-1.7a8.6 8.6 0 0 0 6-6A90 90 0 0 0 68 24a90 90 0 0 0-1.5-16.3z"/>`,
		`<path class="ytembed-tri" d="M27 34l18-10-18-10z"/></svg></span>`,
		`<span class="ytembed-txt"><span class="ytembed-t">`); err != nil {
		return gast.WalkStop, err
	}
	template.HTMLEscape(w, []byte(n.Title))
	if err := write(`</span><span class="ytembed-h">여기서 바로 보기 · 누르면 유튜브에 연결됩니다</span></span></a>` + "\n"); err != nil {
		return gast.WalkStop, err
	}
	return gast.WalkContinue, nil
}
