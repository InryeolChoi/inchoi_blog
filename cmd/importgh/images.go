package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/inryeol/blog/internal/curation"
	"github.com/inryeol/blog/internal/importer"
)

// 저장소의 그림을 DB로 옮긴다.
//
// # 왜 링크로 두지 않나
//
// 본문의 `![](./images/x.png)`를 그대로 두면 그 주소는 **우리 사이트 안의
// 없는 경로**다. raw.githubusercontent.com으로 바꿔 두는 길도 있지만 그러면
// 글을 열 때마다 독자 IP가 GitHub에 가고, 저장소를 지우거나 비공개로 돌리면
// 그림이 통째로 사라진다. 외부 파비콘을 안 불러오기로 한 것과 같은 판단이다
// (CLAUDE.md "남은 일"의 파비콘 항목).
//
// # 노션 쪽과 같은 멱등 키를 쓴다
//
// `images.sha256`이다. 같은 그림을 두 저장소가 갖고 있어도 행이 하나고,
// 파일 이름을 바꿔도 새 그림이 되지 않는다. 저장과 서빙은 노션 이미지가
// 쓰던 것을 그대로 쓴다(`importer.UpsertImage`, `GET /img/{sha256}`).

// mdImage는 본문의 그림 참조다. `![alt](경로)` 꼴만 본다.
//
// **바깥 주소는 건드리지 않는다.** `http://`나 `https://`로 시작하는 것은
// 저장소의 파일이 아니라 남의 그림이라 받아올 대상이 아니다.
var mdImage = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)

// htmlImage는 본문에 그대로 쓴 `<img src="...">`를 찾는다.
//
// **마크다운 문법만 보면 놓친다.** 이 저장소는 가운데 정렬을 하려고
// `<p align="center"><img src="./Images/x.png"></p>` 꼴을 섞어 쓴다.
// 렌더러가 `html.WithUnsafe()`라 그 태그가 그대로 나가는데, 주소를 안 바꾸면
// **우리 사이트 안의 없는 경로**를 가리켜 깨진 그림이 된다. 실제로 14장이
// 그랬다 — 마크다운 43장만 옮기고 리포트는 "못 옮긴 참조 1개"라고 했다.
var htmlImage = regexp.MustCompile(`(<img\s[^>]*?src=")([^"]+)(")`)

// repoImage는 받아온 그림 한 장이다.
type repoImage struct {
	// RepoPath는 저장소 안의 경로다. 같은 그림을 여러 글이 쓰면 한 번만 받는다.
	RepoPath string
	SHA256   string
	Data     []byte
	MIME     string
}

// rewriteImages는 본문의 그림 참조를 `/img/{sha256}`으로 바꾸고, 받아야 할
// 그림 목록을 함께 준다.
//
// `docPath`는 이 글 파일의 저장소 경로다. 그림 경로가 `./images/x.png`처럼
// **그 파일을 기준으로 한 상대경로**라, 여기서 풀어야 저장소 경로가 나온다.
//
// **못 받은 그림은 원문 그대로 둔다.** 주소만 `/img/`로 바꿔 두면 깨진 그림이
// 멀쩡해 보인다 — 죽은 링크를 노션 형태로 남겨 두는 relink의 판단과 같다.
func rewriteImages(body, docPath string, have map[string]repoImage) (string, []string) {
	dir := path.Dir(docPath)
	var want []string
	resolve := func(ref string) (string, bool) {
		if isExternal(ref) {
			return "", false
		}
		repoPath := path.Clean(path.Join(dir, ref))
		img, ok := have[repoPath]
		if !ok {
			want = append(want, repoPath)
			return "", false
		}
		return "/img/" + img.SHA256, true
	}

	out := mdImage.ReplaceAllStringFunc(body, func(m string) string {
		g := mdImage.FindStringSubmatch(m)
		url, ok := resolve(g[2])
		if !ok {
			return m
		}
		return "![" + g[1] + "](" + url + ")"
	})
	out = htmlImage.ReplaceAllStringFunc(out, func(m string) string {
		g := htmlImage.FindStringSubmatch(m)
		url, ok := resolve(g[2])
		if !ok {
			return m
		}
		return g[1] + url + g[3]
	})
	return out, want
}

func isExternal(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") ||
		strings.HasPrefix(ref, "//") || strings.HasPrefix(ref, "data:")
}

// collectImagePaths는 이 글이 쓰는 저장소 안 그림의 경로를 모은다.
func collectImagePaths(body, docPath string) []string {
	dir := path.Dir(docPath)
	var out []string
	add := func(ref string) {
		if isExternal(ref) {
			return
		}
		out = append(out, path.Clean(path.Join(dir, ref)))
	}
	for _, g := range mdImage.FindAllStringSubmatch(body, -1) {
		add(g[2])
	}
	for _, g := range htmlImage.FindAllStringSubmatch(body, -1) {
		add(g[2])
	}
	return out
}

// fetchImage는 저장소에서 그림 하나를 받는다.
//
// **Contents API를 쓴다.** raw 주소와 달리 토큰으로 요청 한도가 올라가고,
// 마크다운을 받아오는 길과 같은 방식이라 인증 처리가 하나로 끝난다.
// 다만 이 API는 1MB가 넘는 파일에 내용을 안 실어 주므로 그때는 download_url을
// 한 번 더 따라간다.
func fetchImage(ctx context.Context, src curation.GitHubSource, repoPath string) (repoImage, error) {
	mime, ok := importer.MIMEForFile(repoPath)
	if !ok {
		return repoImage{}, fmt.Errorf("%s: 모르는 확장자라 MIME을 정할 수 없다", repoPath)
	}
	// **SVG는 받지 않는다.** `/img/{sha256}`이 images.mime을 그대로
	// Content-Type에 실어 보내는데, image/svg+xml은 브라우저가 문서로 열고
	// 그 안의 <script>가 **우리 도메인에서** 실행된다. admin 업로드가 SVG를
	// 막는 것과 같은 이유다.
	if mime == "image/svg+xml" {
		return repoImage{}, fmt.Errorf("%s: SVG는 받지 않는다 (우리 도메인에서 스크립트가 돈다)", repoPath)
	}

	u := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", src.Repo, repoPath)
	if src.Ref != "" {
		u += "?ref=" + src.Ref
	}
	var payload struct {
		Content     string `json:"content"`
		Encoding    string `json:"encoding"`
		DownloadURL string `json:"download_url"`
	}
	if err := getJSON(ctx, u, &payload); err != nil {
		return repoImage{}, fmt.Errorf("%s: %w", repoPath, err)
	}

	var data []byte
	switch payload.Encoding {
	case "base64":
		b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
		if err != nil {
			return repoImage{}, fmt.Errorf("%s 디코딩: %w", repoPath, err)
		}
		data = b
	case "none", "":
		// 1MB가 넘어 내용이 안 실려 왔다. 받아오는 주소를 따로 준다.
		if payload.DownloadURL == "" {
			return repoImage{}, fmt.Errorf("%s: 내용도 download_url도 없다", repoPath)
		}
		b, err := getBytes(ctx, payload.DownloadURL)
		if err != nil {
			return repoImage{}, fmt.Errorf("%s 내려받기: %w", repoPath, err)
		}
		data = b
	default:
		return repoImage{}, fmt.Errorf("%s: 모르는 인코딩 %q", repoPath, payload.Encoding)
	}
	if len(data) == 0 {
		return repoImage{}, fmt.Errorf("%s: 바이트가 없다", repoPath)
	}

	sum := sha256.Sum256(data)
	return repoImage{
		RepoPath: repoPath,
		SHA256:   hex.EncodeToString(sum[:]),
		Data:     data,
		MIME:     mime,
	}, nil
}

func getJSON(ctx context.Context, url string, into any) error {
	res, err := ghGet(ctx, url, "application/vnd.github+json")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(into)
}

func getBytes(ctx context.Context, url string) ([]byte, error) {
	res, err := ghGet(ctx, url, "*/*")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

func ghGet(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	if tok := githubToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("GitHub이 %s를 줬다", res.Status)
	}
	return res, nil
}
