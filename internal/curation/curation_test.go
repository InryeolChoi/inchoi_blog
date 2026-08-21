package curation

import "testing"

func TestRemoveLineCollapsesBlank(t *testing.T) {
	body := "앞 문단\n\n[프로젝트](/p/abc)\n"
	got, ok := removeLine(body, "[프로젝트](/p/abc)")
	if !ok {
		t.Fatal("줄을 못 찾았다")
	}
	// 빈 줄이 둘 남으면 문단 사이가 벌어진다.
	if want := "앞 문단\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRemoveLineInMiddle(t *testing.T) {
	body := "앞\n\n지울 줄\n\n뒤\n"
	got, ok := removeLine(body, "지울 줄")
	if !ok {
		t.Fatal("줄을 못 찾았다")
	}
	if want := "앞\n\n뒤\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 줄 전체가 맞아야 한다. 조각만 같은 줄을 지우면 앞뒤가 어떻게 이어지는지
// 표만 보고는 알 수 없다.
func TestRemoveLineNeedsWholeLine(t *testing.T) {
	if _, ok := removeLine("여기 [프로젝트](/p/abc) 있다\n", "[프로젝트](/p/abc)"); ok {
		t.Error("문장 속 조각을 지웠다")
	}
}

func TestRemoveLineIgnoresTrailingSpace(t *testing.T) {
	if _, ok := removeLine("앞\n\n지울 줄   \n\n뒤\n", "지울 줄"); !ok {
		t.Error("뒤에 공백이 붙은 줄을 못 찾았다")
	}
}

func TestApplyBodyEditsLeavesOtherPagesAlone(t *testing.T) {
	body := "[프로젝트](/p/fd9d12dc-83de-4424-9428-0f26582130bc)\n"
	got, err := ApplyBodyEdits("다른-페이지", body)
	if err != nil {
		t.Fatalf("ApplyBodyEdits: %v", err)
	}
	if got != body {
		t.Errorf("남의 글을 고쳤다: %q", got)
	}
}

// 표가 낡았는데 조용히 넘어가면, 지웠다고 믿는 것이 본문에 그대로 남는다.
func TestApplyBodyEditsFailsWhenLineMissing(t *testing.T) {
	if len(BodyEdits) == 0 {
		t.Skip("BodyEdits가 비어 있다")
	}
	e := BodyEdits[0]
	if _, err := ApplyBodyEdits(e.NotionPageID, "그 줄이 없는 본문\n"); err == nil {
		t.Error("못 찾았는데 에러가 아니다")
	}
}

// 실제 표가 도는지 본다. 소개 글 끝의 인라인 데이터베이스 링크를 덜어낸다.
func TestApplyBodyEditsOnIntroPost(t *testing.T) {
	const intro = "1080901b-87f1-80d2-811a-eba467c2c160"
	body := "![](/img/0f9f83dcd63eb36d2bbc1c616342d8a8d2edfc29b6ba318debc159bcbf336128)\n\n" +
		"멈추지 않고 끊임없이 나아가는 개발자가 되고 싶습니다.\n\n" +
		"[프로젝트](/p/fd9d12dc-83de-4424-9428-0f26582130bc)\n"
	got, err := ApplyBodyEdits(intro, body)
	if err != nil {
		t.Fatalf("ApplyBodyEdits: %v", err)
	}
	want := "멈추지 않고 끊임없이 나아가는 개발자가 되고 싶습니다.\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 표의 키가 실제로 다른 표와 같은 글을 가리키는지 본다. 소개 글은 intro
// 분류의 표지이기도 하다 — 둘이 어긋나면 한쪽이 낡은 것이다.
func TestBodyEditPageIDsAreKnown(t *testing.T) {
	covers := CoverPageIDs()
	for id := range BodyEditPageIDs() {
		if !covers[id] {
			t.Logf("표지 글이 아닌 글의 본문을 고친다: %s (문제는 아니다)", id)
		}
	}
}

func TestPortraitIsDroppedFromBodyAndImageImport(t *testing.T) {
	const sha = "0f9f83dcd63eb36d2bbc1c616342d8a8d2edfc29b6ba318debc159bcbf336128"
	if !DroppedImage(sha) {
		t.Error("자기소개 사진이 이미지 이관 제외 대상이 아니다")
	}
}
