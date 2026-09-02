package curation

// 네트워크 표지 글의 본문 손질(2026-09-02).
//
// # 무엇이 겹쳤나
//
// 표지 본문이 입구 글 넷을 목록으로 안내하는데, 그중 셋은 같은 이름의 하위
// 분류가 따로 있었다. 그래서 한 화면에 같은 이름이 두 벌로 나왔다 —
// 위에는 본문의 링크, 아래에는 `하위 분류` 상자.
//
// 겹침을 없애는 장치는 이미 있다(`linkCoveredChildCategories`). 본문의 링크가
// **하위 분류의 표지 글**을 가리키면 그 링크를 분류 URL로 바꾸고 그 분류를
// 아래 목록에서 뺀다. 그런데 이 셋은 `cover_post_id`가 비어 있어서 그 장치가
// 걸리지 않았다. 그래서 `PostMoves`로 각 분류에 내려보내고 `Covers`로 표지를
// 지정했다. 셋 중 둘은 draft라 `StatusEdits`로 함께 올렸다 —
// `store.coverSlug`가 숨긴 글을 걸러내므로 draft인 표지는 없는 것과 같다.
//
// 여기 남은 것은 그 넷 중 **본문에서 손봐야 하는 두 줄**이다.
var networkEdits = []BodyEdit{
	{
		NotionPageID: "7929bb22-3973-4ee9-b526-fea976f91ed4",
		Remove:       "[네트워크 이론 (1) ](/p/55abb689-6d32-4c4f-9949-94b868e6aaec)",
		Replace:      "[네트워크 이론](/p/55abb689-6d32-4c4f-9949-94b868e6aaec)",
		Title:        "네트워크",
		Why: "`(2)`가 없어졌으므로 `(1)`이라는 번호도 뜻을 잃었다. " +
			"글 제목은 PostTitleEdits가, 분류 이름은 regroup의 renames가 함께 바꾼다",
	},
	{
		NotionPageID: "7929bb22-3973-4ee9-b526-fea976f91ed4",
		Remove:       "[네트워크 이론 (2)](/p/14e265f9-f285-4302-905e-42783a4a769c)",
		Title:        "네트워크",
		Why: "가리키는 글을 DropPosts로 뺐다. 남겨두면 대상이 posts에 없는 slug가 되어 " +
			"눌러도 404이고, 렌더러가 그걸 노션 인라인 데이터베이스로 알아봐 " +
			"엉뚱한 목록을 펼 수 있다",
	},
}
