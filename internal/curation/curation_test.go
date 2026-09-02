package curation

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

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

func TestApplyBodyEditsRenamesMathStatsTwoReferenceLinks(t *testing.T) {
	const pageID = "59d18904-4ed5-4f0a-ba61-17ec86d9fc7b"
	body := "## 참고자료\n\n" +
		"[분포별 가능도함수 (1)](/p/df333507-a679-4beb-b165-285ae3bf42fc)\n\n" +
		"[확률함수와 커널 (1)](/p/805825dd-084e-4612-9465-a2054a0d2004)\n\n" +
		"[수리통계2 - 과제 (1)](/p/d9fe0a39-89cd-48b6-84ee-0efaa78cf67b)\n\n" +
		"[수리통계2 - 시험 (1)](/p/1f3d0731-e367-4d0d-8239-94d92d6d02d5)\n"
	got, err := ApplyBodyEdits(pageID, body)
	if err != nil {
		t.Fatalf("ApplyBodyEdits: %v", err)
	}
	if strings.Contains(got, " (1)]") {
		t.Errorf("링크 문구에 (1)이 남았다:\n%s", got)
	}
	for _, want := range []string{
		"[분포별 가능도함수](/p/df333507-a679-4beb-b165-285ae3bf42fc)",
		"[확률함수와 커널](/p/805825dd-084e-4612-9465-a2054a0d2004)",
		"[수리통계2 - 과제](/p/d9fe0a39-89cd-48b6-84ee-0efaa78cf67b)",
		"[수리통계2 - 시험](/p/1f3d0731-e367-4d0d-8239-94d92d6d02d5)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("바뀐 링크가 없다: %s", want)
		}
	}
}

func TestAppendBodyIsIdempotent(t *testing.T) {
	const appendix = "## 해답\n\n<details>\n<summary>해답 보기</summary>\n\n풀이\n\n</details>"
	got, err := appendBody("문제\n", "## 해답", appendix)
	if err != nil {
		t.Fatalf("appendBody: %v", err)
	}
	if strings.Count(got, "## 해답") != 1 {
		t.Fatalf("해답이 한 번 붙지 않았다:\n%s", got)
	}
	again, err := appendBody(got, "## 해답", appendix)
	if err != nil {
		t.Fatalf("두 번째 appendBody: %v", err)
	}
	if again != got {
		t.Errorf("두 번째 적용에서 본문이 바뀌었다:\n%s", again)
	}
	if _, err := appendBody("문제\n\n## 해답\n\n다른 풀이\n", "## 해답", appendix); err == nil {
		t.Error("같은 marker의 다른 내용이 있는데 통과했다")
	}
}

func TestProbabilityProcessExamSolutionsAreCollapsible(t *testing.T) {
	const pageID = "eb9b0b6c-697e-4ebe-b8b5-34a44e59095f"
	body := "**중간고사**\n\n![](/img/midterm)\n\n**기말고사**\n\n![](/img/final)\n"
	got, err := ApplyBodyEdits(pageID, body)
	if err != nil {
		t.Fatalf("ApplyBodyEdits: %v", err)
	}
	for _, want := range []string{
		"<summary><strong>중간고사 해답 보기</strong></summary>",
		"<summary><strong>기말고사 해답 보기</strong></summary>",
		`\frac{8263}{180000}`,
		`\operatorname{Var}(Z_3)=1875`,
		`-\frac{41}{160}`,
		`P(X_\infty=2)=\pi_2=\frac49`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("해답에 %q가 없다", want)
		}
	}
	if strings.Count(got, "<details>") != 2 || strings.Count(got, "</details>") != 2 {
		t.Errorf("중간·기말 접기 영역이 정확히 두 개가 아니다")
	}
	if strings.Count(got, "## 중간고사 해답") != 1 {
		t.Errorf("중간고사 해답 marker가 한 번이 아니다")
	}
}

func TestMachineLearningCoverDropsBrokenHereAndThereLink(t *testing.T) {
	const pageID = "226b7998-bd88-4892-88aa-1227dc89b5f0"
	body := "- 통계와 수학에 관한 지식이 많이 필요하다. 모르면 [여기, 저기](/0d24cebd0bc1496b99b64885cb5be2a6) [클릭!](/5d2a5e480d854fc594d3280cbeef87ee)\n\n" +
		"## 연습문제 2\n\n[교차검증과 과대적합](/p/358b2929-84e3-406f-8575-0e19534153d0)\n"
	got, err := ApplyBodyEdits(pageID, body)
	if err != nil {
		t.Fatalf("ApplyBodyEdits: %v", err)
	}
	want := "- 통계와 수학에 관한 지식이 많이 필요하다. 모르면 [클릭!](/5d2a5e480d854fc594d3280cbeef87ee)"
	if !strings.Contains(got, want) {
		t.Errorf("정리한 링크 문장이 없다:\n%s", got)
	}
	if strings.Contains(got, "여기, 저기") || strings.Contains(got, "/0d24cebd0bc1496b99b64885cb5be2a6") {
		t.Errorf("깨진 링크가 남았다:\n%s", got)
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

func TestEcole42EntryPostsBecomeChildCategoryCovers(t *testing.T) {
	want := map[string]string{
		"c":              "c7bcee75-28c5-4945-b5c3-f8e24e79e5e7",
		"cpp-part-1":     "5c84aeb4-c3d9-4341-a66b-acc71487be94",
		"netpractice":    "0016e85a-614f-426c-ae62-f46427a7b719",
		"shell":          "5b239f6c-32bb-4a4a-921b-00db117abc3d",
		"born2beroot":    "c4e6e521-2c48-460b-91d8-54d8452f2096",
		"cub3d":          "92ab7921-feb3-4604-b9ce-171a3b8a4629",
		"exam02":         "90a0bf6c-e299-4158-8e13-d367d96298e5",
		"exam03":         "96b38479-74a3-4983-a9d9-c5530a9b94c8",
		"fdf-fil-de-fer": "f6ceedec-0c40-4a30-8b97-05c05868e6f7",
		"ft-irc-server":  "97c6e452-91e6-4c19-9769-802b6be9a982",
		"ft-printf":      "62d53e5d-77fb-4a4e-b789-9a59ef3a70e4",
		"get-next-line":  "38315c40-5daf-4387-bae3-bde27387b43f",
		"inception":      "4786e824-c744-4c32-8886-729e8e8c9bc6",
		"libft":          "dcbbd10e-b50a-4a1d-8a4a-237426aa7249",
		"minishell":      "518ea6e1-306c-4cfa-8b3e-6ac182c16e14",
		"philosopher":    "cdf4ecb8-ecdb-4cda-9dbc-d74722213449",
		"pipex":          "250631b1-6ac8-4e58-8dc2-eec4ecaca254",
	}
	moves := PostMoveBySlug()
	covers := make(map[string]string, len(Covers))
	for _, cover := range Covers {
		if _, exists := covers[cover.Slug]; exists {
			t.Errorf("중복 표지 slug: %s", cover.Slug)
		}
		covers[cover.Slug] = cover.NotionPageID
	}
	for slug, id := range want {
		if got := moves[id]; got != slug {
			t.Errorf("%s 이동 목적지=%q, want %q", id, got, slug)
		}
		if got := covers[slug]; got != id {
			t.Errorf("%s 표지=%q, want %q", slug, got, id)
		}
	}
}

// **묶음마다 규칙이 다르다.** 선형대수는 제목에 번호를 붙이고 날짜까지 사람이
// 정했지만, 다변량분석은 순서만 정하고 제목·날짜는 원본 그대로다. 그래서 표
// 전체에 한 가지 규칙을 걸 수 없다 — 묶음별로 따로 보고, 표 전체에는 겹치는
// 것이 없다는 것만 본다.
func TestLinearAlgebraMetadataIsCompleteAndOrdered(t *testing.T) {
	if got, want := len(linearAlgebraMetadataEdits), 11; got != want {
		t.Fatalf("선형대수 메타데이터가 %d건이다, want %d", got, want)
	}
	moves := PostMoveBySlug()
	seen := map[string]bool{}
	start := time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2022, 8, 31, 23, 59, 59, 0, time.UTC)
	for i, edit := range linearAlgebraMetadataEdits {
		if seen[edit.NotionPageID] {
			t.Errorf("중복 notion_page_id: %s", edit.NotionPageID)
		}
		seen[edit.NotionPageID] = true
		wantPrefix := fmt.Sprintf("%d. ", i+1)
		if len(edit.Title) < len(wantPrefix) || edit.Title[:len(wantPrefix)] != wantPrefix {
			t.Errorf("%d번째 제목이 번호로 시작하지 않는다: %q", i+1, edit.Title)
		}
		if edit.SortOrder != i {
			t.Errorf("%q sort_order=%d, want %d", edit.Title, edit.SortOrder, i)
		}
		date, err := time.Parse("2006-01-02", edit.OriginalCreatedAt)
		if err != nil {
			t.Errorf("%q 날짜 파싱: %v", edit.Title, err)
			continue
		}
		if date.Before(start) || date.After(end) {
			t.Errorf("%q 날짜가 2022년 3~8월 밖이다: %s", edit.Title, date)
		}
		if moves[edit.NotionPageID] != "선형대수" {
			t.Errorf("%q가 선형대수 이동 대상이 아니다", edit.Title)
		}
	}
	if got := linearAlgebraMetadataEdits[3].Title; got != "4. 벡터공간" {
		t.Errorf("벡터공간 번호가 다르다: %q", got)
	}
}

// 다변량분석은 **순서만** 사람이 정했다. 제목과 작성일을 건드리면 안 된다 —
// 원본 그대로여야 재이관 때 노션 값이 그대로 살아난다.
func TestMultivariateCodeOrderTouchesOnlyTheOrder(t *testing.T) {
	want := []string{
		"다변량분석?", "확률분포와 자료행렬",
		"주성분분석 : 코드 (1)", "주성분분석 : 코드 (2)",
		"인자분석 : 코드 (1)", "인자분석 : 코드 (2)",
		"인자분석 : 코드 (3)", "인자분석 : 코드 (4)",
		"정준분석 : 코드 ", "대응분석 : 코드",
		"군집분석 : 코드", "판별분석 : 코드",
	}
	if got := len(multivariateCodeOrderEdits); got != len(want) {
		t.Fatalf("다변량분석 메타데이터가 %d건이다, want %d", got, len(want))
	}
	for i, edit := range multivariateCodeOrderEdits {
		if edit.Title != want[i] {
			t.Errorf("%d번째 제목이 %q다, want %q", i, edit.Title, want[i])
		}
		// **제목을 바꾸지 않는다.** 둘이 다르면 사람이 제목까지 손댄 것이다.
		if edit.Title != edit.OriginalTitle {
			t.Errorf("%q: 제목을 바꾸고 있다 (원본 %q)", edit.Title, edit.OriginalTitle)
		}
		// **작성일을 비워야 노션 원본이 유지된다.** 값을 적으면 그날로 덮인다.
		if edit.OriginalCreatedAt != "" {
			t.Errorf("%q: 작성일 %q를 적었다. 순서만 정하는 묶음이다",
				edit.Title, edit.OriginalCreatedAt)
		}
		if edit.SortOrder != i {
			t.Errorf("%q sort_order=%d, want %d", edit.Title, edit.SortOrder, i)
		}
	}
}

// 표를 합쳤으므로 **묶음끼리 같은 글을 두 번 잡지 않는지**는 따로 봐야 한다.
// 겹치면 PostMetadataByID가 조용히 한쪽을 이긴다.
func TestMetadataEditsHaveNoDuplicates(t *testing.T) {
	seen := map[string]string{}
	for _, edit := range PostMetadataEdits {
		if prev, dup := seen[edit.NotionPageID]; dup {
			t.Errorf("%s를 두 번 잡는다: %q와 %q", edit.NotionPageID, prev, edit.Title)
		}
		seen[edit.NotionPageID] = edit.Title
	}
	if got, want := len(PostMetadataEdits),
		len(linearAlgebraMetadataEdits)+len(multivariateCodeOrderEdits)+len(optimizationPracticeOrderEdits); got != want {
		t.Errorf("합친 표가 %d건이다, want %d", got, want)
	}
}

func TestOptimizationPostsStayOutOfLinearAlgebra(t *testing.T) {
	moves := PostMoveBySlug()
	for _, id := range []string{
		"404c96b3-e53c-4edb-88ee-8ef0f717ce79",
		"ad882859-b0f2-41d9-9552-c7c14cf0b559",
		"ff7a1343-68d1-4465-9df0-ea48a0a2565b",
		"e96b9abf-d1de-4790-8656-7ba4a57c4d89",
		"eedb3add-e5e1-4b8a-a1d3-41bc80e00162",
	} {
		if got := moves[id]; got != "최적화이론" {
			t.Errorf("최적화 글 %s의 목적지가 %q다", id, got)
		}
	}
}

func TestApplyPostMetadata(t *testing.T) {
	edit := PostMetadataEdits[3]
	original := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	title, created, order, err := ApplyPostMetadata(edit.NotionPageID, edit.OriginalTitle, &original)
	if err != nil {
		t.Fatalf("ApplyPostMetadata: %v", err)
	}
	if title != "4. 벡터공간" || created.Format("2006-01-02") != "2022-04-23" || *order != 3 {
		t.Errorf("got title=%q date=%s order=%d", title, created.Format("2006-01-02"), *order)
	}
	if _, _, _, err := ApplyPostMetadata(edit.NotionPageID, "엉뚱한 제목", &original); err == nil {
		t.Error("원본 제목이 다른데 통과했다")
	}
}

func TestMathStatsTwoReferenceTitlesDropDuplicateSuffix(t *testing.T) {
	// **건수를 못박지 않는다.** 이 표는 수리통계2 넷으로 시작했지만 제목만
	// 바꾸는 예외는 다른 글에서도 생긴다(네트워크 이론). 여기서 지키는 것은
	// 건수가 아니라 아래 세 가지다 — 중복 없음, 제목에 `(1)`이 안 남음,
	// 작성일과 순서를 건드리지 않음.
	if len(PostTitleEdits) == 0 {
		t.Fatal("제목만 바꾸는 표가 비었다")
	}
	seen := map[string]bool{}
	for _, edit := range PostTitleEdits {
		if seen[edit.NotionPageID] {
			t.Errorf("중복 notion_page_id: %s", edit.NotionPageID)
		}
		seen[edit.NotionPageID] = true
		if strings.HasSuffix(edit.Title, " (1)") {
			t.Errorf("수정 제목에 (1)이 남았다: %q", edit.Title)
		}

		original := time.Date(2023, 7, 22, 14, 26, 0, 0, time.UTC)
		title, created, order, err := ApplyPostMetadata(edit.NotionPageID, edit.OriginalTitle, &original)
		if err != nil {
			t.Fatalf("ApplyPostMetadata(%q): %v", edit.OriginalTitle, err)
		}
		if title != edit.Title || !created.Equal(original) || order != nil {
			t.Errorf("title=%q created=%v order=%v", title, created, order)
		}
	}
}

// 묶음 이름은 표에 손으로 적지 않고 concatMetadataEdits가 찍는다.
// **이름이 다르지 않으면 두 묶음의 0번이 같은 자리를 다투는 것으로 보인다**
// (cmd/sortorder의 중복 검사).
func TestMetadataGroupsAreNamedApart(t *testing.T) {
	groups := map[string]int{}
	for _, edit := range PostMetadataEdits {
		if edit.Group == "" {
			t.Fatalf("%q에 묶음 이름이 없다", edit.Title)
		}
		groups[edit.Group]++
	}
	if len(groups) < 2 {
		t.Fatalf("묶음이 %d개다. 둘 이상이어야 이 검사가 뜻이 있다", len(groups))
	}
	// 묶음 안에서 sort_order가 0부터 빠짐없이 이어져야 한다.
	for name := range groups {
		seen := map[int]bool{}
		for _, edit := range PostMetadataEdits {
			if edit.Group != name {
				continue
			}
			if seen[edit.SortOrder] {
				t.Errorf("%s: sort_order %d가 겹친다", name, edit.SortOrder)
			}
			seen[edit.SortOrder] = true
		}
		for i := 0; i < groups[name]; i++ {
			if !seen[i] {
				t.Errorf("%s: sort_order %d가 비었다", name, i)
			}
		}
	}
}
