package markdown

import (
	gast "github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// dropEmptyTableHead는 칸이 전부 빈 표 머리를 없앤다.
//
// **GFM 표는 머리 행이 문법상 필수다.** 노션 표에는 머리가 없는 경우가 많아서
// (덤프의 표 79개 중 64개) 변환기가 빈 머리를 넣어 형태를 맞춘다
// (`KindTableNoHeader`). 그대로 그리면 표 위에 아무것도 없는 회색 띠가 한 줄
// 생긴다 — 내용이 아니라 문법을 맞추려고 넣은 것이니 화면에서는 뺀다.
//
// **파서 단계에서 뺀다.** 결과 HTML을 문자열로 손보는 것보다, 표가 어떻게
// 생겼는지 아는 자리에서 판단하는 편이 정확하다.
type dropEmptyTableHead struct{}

func (dropEmptyTableHead) Transform(doc *gast.Document, reader text.Reader, _ parser.Context) {
	var heads []*east.TableHeader
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if h, ok := n.(*east.TableHeader); ok && allCellsEmpty(h, reader) {
			heads = append(heads, h)
		}
		return gast.WalkContinue, nil
	})
	// 걷는 중에 지우면 순회가 꼬인다. 다 모은 뒤에 뗀다.
	for _, h := range heads {
		if p := h.Parent(); p != nil {
			p.RemoveChild(p, h)
		}
	}
}

// allCellsEmpty는 머리 칸이 전부 비었는지 본다. 한 칸이라도 글자가 있으면
// 사람이 쓴 머리이므로 그대로 둔다.
func allCellsEmpty(h *east.TableHeader, reader text.Reader) bool {
	if h.ChildCount() == 0 {
		return false
	}
	for c := h.FirstChild(); c != nil; c = c.NextSibling() {
		cell, ok := c.(*east.TableCell)
		if !ok {
			return false
		}
		if cell.ChildCount() > 0 || len(cell.Text(reader.Source())) > 0 {
			return false
		}
	}
	return true
}
