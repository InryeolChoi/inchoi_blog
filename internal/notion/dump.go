// Package notion은 scripts/dump/의 노션 덤프를 읽어 마크다운으로 변환한다.
//
// 덤프는 노션 API 응답을 그대로 저장한 것이라 블록마다 본문 필드 이름이 다르다
// (type이 "paragraph"면 본문은 "paragraph" 키에 들어 있다). 그래서 Block은
// 본문을 원본 JSON으로 들고 있다가 타입별로 따로 디코딩한다.
package notion

import (
	"encoding/json"
	"fmt"
	"os"
)

// Dump는 scripts/dump/{page_id}.json 파일 하나에 대응한다.
type Dump struct {
	Page   Page    `json:"page"`
	Blocks []Block `json:"blocks"`
}

// Parent는 페이지가 어디에 속해 있는지다.
//
// Type은 "page_id" | "database_id" | "block_id" | "workspace" 중 하나다.
// 형제 순서를 어디서 얻을 수 있는지가 여기서 갈린다: 부모가 페이지나 블록이면
// 그쪽 blocks 배열에 순서가 있지만, 데이터베이스면 노션 API가 순서를 주지 않는다.
type Parent struct {
	Type       string `json:"type"`
	PageID     string `json:"page_id"`
	DatabaseID string `json:"database_id"`
	BlockID    string `json:"block_id"`
}

// Page는 노션 GET /v1/pages/{id} 응답 중 이관에 쓰는 부분이다.
type Page struct {
	ID             string                     `json:"id"`
	CreatedTime    string                     `json:"created_time"`
	LastEditedTime string                     `json:"last_edited_time"`
	URL            string                     `json:"url"`
	Parent         Parent                     `json:"parent"`
	Properties     map[string]json.RawMessage `json:"properties"`
}

// Title은 페이지 제목을 돌려준다.
//
// 제목 프로퍼티의 이름은 페이지마다 다르다("이름", "title", "Name", "단원" 등).
// 이름 대신 type이 "title"인 프로퍼티를 찾는다.
func (p Page) Title() string {
	for _, raw := range p.Properties {
		var prop struct {
			Type  string     `json:"type"`
			Title []RichText `json:"title"`
		}
		if err := json.Unmarshal(raw, &prop); err != nil {
			continue
		}
		if prop.Type == "title" {
			return PlainText(prop.Title)
		}
	}
	return ""
}

// Block은 노션 블록 하나다. Body에는 타입 이름과 같은 키의 원본 JSON이 들어간다.
type Block struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	HasChildren bool    `json:"has_children"`
	Children    []Block `json:"children"`

	// Body는 블록 본문의 원본 JSON이다(type이 "code"면 "code" 키의 값).
	Body json.RawMessage `json:"-"`
}

func (b *Block) UnmarshalJSON(data []byte) error {
	type blockAlias Block // 재귀 호출을 피하려고 메서드 없는 타입으로 한 번 감싼다.
	var alias blockAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*b = Block(alias)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	b.Body = fields[b.Type]
	return nil
}

// decodeBody는 블록 본문을 목표 타입으로 디코딩한다. 본문이 없으면 아무것도 하지 않는다.
func (b Block) decodeBody(target any) error {
	if len(b.Body) == 0 {
		return nil
	}
	if err := json.Unmarshal(b.Body, target); err != nil {
		return fmt.Errorf("블록 %s(%s) 본문 디코딩: %w", b.ID, b.Type, err)
	}
	return nil
}

// RichText는 노션의 서식 있는 텍스트 조각 하나다.
type RichText struct {
	Type      string `json:"type"`
	PlainText string `json:"plain_text"`
	Href      string `json:"href"`

	Annotations struct {
		Bold          bool `json:"bold"`
		Italic        bool `json:"italic"`
		Strikethrough bool `json:"strikethrough"`
		Underline     bool `json:"underline"`
		Code          bool `json:"code"`
	} `json:"annotations"`

	Equation *struct {
		Expression string `json:"expression"`
	} `json:"equation"`
}

// PlainText는 서식을 뺀 순수 텍스트를 이어붙인다. 길이 비교용이다.
func PlainText(rts []RichText) string {
	var out string
	for _, rt := range rts {
		out += rt.PlainText
	}
	return out
}

// LoadDump는 덤프 파일 하나를 읽는다.
func LoadDump(path string) (*Dump, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("덤프 읽기(%s): %w", path, err)
	}
	var d Dump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("덤프 파싱(%s): %w", path, err)
	}
	return &d, nil
}

// ImageSources는 저장된 이미지의 sha256 → 노션 원본 URL 대응을 모은다.
//
// 노션이 호스팅하는 파일 URL(file.url)은 서명이 붙은 임시 주소라 곧 만료된다.
// 그래도 어디서 온 이미지인지 기록으로 남겨둔다.
func (d *Dump) ImageSources() map[string]string {
	out := map[string]string{}
	var walk func(blocks []Block)
	walk = func(blocks []Block) {
		for _, b := range blocks {
			if b.Type == "image" {
				var body struct {
					Local *struct {
						SHA256 string `json:"sha256"`
					} `json:"local"`
					File     *struct{ URL string } `json:"file"`
					External *struct{ URL string } `json:"external"`
				}
				if err := b.decodeBody(&body); err == nil &&
					body.Local != nil && body.Local.SHA256 != "" {
					url := ""
					if body.External != nil {
						url = body.External.URL
					} else if body.File != nil {
						url = body.File.URL
					}
					if url != "" {
						out[body.Local.SHA256] = url
					}
				}
			}
			walk(b.Children)
		}
	}
	walk(d.Blocks)
	return out
}

// ChildPageSibling은 하위 페이지 하나가 형제들 사이에서 몇 번째인지다.
type ChildPageSibling struct {
	PageID string
	// ContainerID는 이 하위 페이지를 담고 있는 페이지 또는 블록의 id다.
	ContainerID string
	// Index는 같은 컨테이너 안의 child_page들 사이에서 0부터 세는 순번이다.
	Index int
}

// ChildPageOrder는 이 덤프에 담긴 하위 페이지들의 형제 순서를 돌려준다.
//
// 노션 API는 블록을 화면에 보이는 순서 그대로 배열로 준다. 그래서 어떤 페이지의
// blocks 배열에서 child_page 블록이 나오는 차례가 곧 하위 페이지의 순서다.
// child_page 아닌 블록(문단, 이미지 등)은 세지 않는다.
func (d *Dump) ChildPageOrder() []ChildPageSibling {
	var out []ChildPageSibling

	var walk func(blocks []Block, containerID string)
	walk = func(blocks []Block, containerID string) {
		idx := 0
		for _, b := range blocks {
			if b.Type == "child_page" {
				out = append(out, ChildPageSibling{
					PageID:      b.ID, // child_page 블록의 id가 곧 그 하위 페이지의 id다
					ContainerID: containerID,
					Index:       idx,
				})
				idx++
			}
			// 토글이나 단 안에 하위 페이지가 들어 있을 수 있다. 그 경우 형제 범위는
			// 그 블록 안이므로 컨테이너를 바꿔서 따로 센다.
			if len(b.Children) > 0 {
				walk(b.Children, b.ID)
			}
		}
	}
	walk(d.Blocks, d.Page.ID)
	return out
}
