package curation

// softWrapEdits는 노션에서 긴 문장을 Enter(줄바꿈, shift+Enter가 아니다)로 감싸
// 쓴 자리를 한 줄로 합친다.
//
// **왜 지금 와서야 문제가 됐나.** 렌더러가 CommonMark 기본대로 "줄 끝에 공백
// 두 개가 있어야 <br>" 규칙을 따르는 동안은, 이 문단 내부 줄바꿈이 HTML의
// 보통 공백으로 접혀 화면에서 안 보였다. 렌더러를 Enter 한 번마다 <br>을 내는
// 쪽으로 바꾸면서(internal/markdown/render.go의 html.WithHardWraps) 그동안
// 숨어 있던 이 흔적이 문장 한가운데서 튀는 줄바꿈으로 드러났다.
//
// **한 줄만 지우고 바꾸는 BodyEdit로는 두 줄을 하나로 합칠 수 없다** —
// replaceLine/removeLine이 정확히 한 줄만 본다. 그래서 각 자리를 둘로 나눈다:
// 첫 줄은 합친 전체 문장으로 Replace하고, 나머지 줄은 Remove한다. 셋 이상을
// 합칠 때도 같다 — 첫 줄만 Replace, 그 뒤는 모두 Remove.
//
// **한 자리는 일부러 뺐다.** id 490(핸즈온 머신러닝 원-핫 인코딩 예시)의
// "1H OCEAN 이면 한 특성이 1, 그 외 0" / "inland 이면 한 특성이 0, 그 외 1"은
// 줄바꿈처럼 보이지만 실은 서로 다른 두 문장이라 합치면 뜻이 바뀐다.
var softWrapEdits = buildSoftWrapEdits()

func buildSoftWrapEdits() []BodyEdit {
	return []BodyEdit{
		{
			NotionPageID: "07063360-cd9a-4164-a5d1-2ac8d0263f61",
			Remove:       "각 **포트**에는 **포트**와 상호작용할 자격이 되는지 식별하는 <u>**포트 권한 집합**</u>이",
			Replace:      "각 **포트**에는 **포트**와 상호작용할 자격이 되는지 식별하는 <u>**포트 권한 집합**</u>이 연관됨.",
			Title:        "프로세스 간 통신 (3) : 메세지 전달 (예시)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "07063360-cd9a-4164-a5d1-2ac8d0263f61",
			Remove:       "연관됨.",
			Title:        "프로세스 간 통신 (3) : 메세지 전달 (예시)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "0f4a0c87-a897-4a6e-9b24-bc172f8aa0a0",
			Remove:       "다익스트라 알고리즘은 특정 정점에서 인접한 정점을 가중치가 작은 순으로",
			Replace:      "다익스트라 알고리즘은 특정 정점에서 인접한 정점을 가중치가 작은 순으로 큐에 저장해가며 거리가 짧은 경로를 먼저 추출한다.",
			Title:        "다익스트라 알고리즘",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "0f4a0c87-a897-4a6e-9b24-bc172f8aa0a0",
			Remove:       "큐에 저장해가며 거리가 짧은 경로를 먼저 추출한다.",
			Title:        "다익스트라 알고리즘",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "1a4d935e-271e-4a32-b533-1c65db8e5ac8",
			Remove:       "   예를 들어 ft_strdup()을 만든다고 가정하면, ft_strlen()을 안 만들어도 그 껍데기만 가지고",
			Replace:      "   예를 들어 ft_strdup()을 만든다고 가정하면, ft_strlen()을 안 만들어도 그 껍데기만 가지고 일단 설계하고, 나중에 ft_strlen()을 만들면 된다.",
			Title:        "인터페이스",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "1a4d935e-271e-4a32-b533-1c65db8e5ac8",
			Remove:       "   일단 설계하고, 나중에 ft_strlen()을 만들면 된다.",
			Title:        "인터페이스",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "260e6fbf-188f-4c20-bf59-0e644abd190d",
			Remove:       "위의 페이지는 `ligthttpd.conf` 파일의 `server.document-root` 에 해당하는",
			Replace:      "위의 페이지는 `ligthttpd.conf` 파일의 `server.document-root` 에 해당하는 디렉토리인 `/var/www/html/` 의 `index.lighttpd.html` 파일을 보여준 것이다.",
			Title:        "born2beroot - 보너스 (1)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "260e6fbf-188f-4c20-bf59-0e644abd190d",
			Remove:       "디렉토리인 `/var/www/html/` 의 `index.lighttpd.html` 파일을 보여준 것이다.",
			Title:        "born2beroot - 보너스 (1)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "2b123c28-4395-4e49-a4ac-c703f046e2f8",
			Remove:       "따라서, 입력받은 수가 $1 + \\displaystyle\\sum_{i=1}^{n-1} 6n$ 보다 작거나 같은 경우",
			Replace:      "따라서, 입력받은 수가 $1 + \\displaystyle\\sum_{i=1}^{n-1} 6n$ 보다 작거나 같은 경우 i+1번째라고  생각하면 풀린다.",
			Title:        "2292번: 벌집",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "2b123c28-4395-4e49-a4ac-c703f046e2f8",
			Remove:       "i+1번째라고  생각하면 풀린다.",
			Title:        "2292번: 벌집",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "317cf497-b5f4-4535-89e2-08bb66071639",
			Remove:       "한수는 크기가 $2^N × 2^N$인 2차원 배열을 Z모양으로",
			Replace:      "한수는 크기가 $2^N × 2^N$인 2차원 배열을 Z모양으로 탐색하려고 한다.",
			Title:        "1074번: Z",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "317cf497-b5f4-4535-89e2-08bb66071639",
			Remove:       "탐색하려고 한다.",
			Title:        "1074번: Z",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "54d36397-d8f2-4364-bbe2-4244848dcc19",
			Remove:       "$\\hat\\theta$를 사용해 예측",
			Replace:      "$\\hat\\theta$를 사용해 예측 $\\hat y = X\\hat\\theta$",
			Title:        "4. 회귀모델 훈련",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "54d36397-d8f2-4364-bbe2-4244848dcc19",
			Remove:       "$\\hat y = X\\hat\\theta$",
			Title:        "4. 회귀모델 훈련",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "59648a0f-6a39-426f-8f87-b6bf1974c56d",
			Remove:       "패키지 관리자를 사용하면 사용자는 수동으로 소프트웨어를 설치, 업그레이드, 제거하는 번거로움 없이 손쉽게",
			Replace:      "패키지 관리자를 사용하면 사용자는 수동으로 소프트웨어를 설치, 업그레이드, 제거하는 번거로움 없이 손쉽게 소프트웨어를 관리할 수 있음.",
			Title:        "born2beroot - 패키지 관리자",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "59648a0f-6a39-426f-8f87-b6bf1974c56d",
			Remove:       "소프트웨어를 관리할 수 있음.",
			Title:        "born2beroot - 패키지 관리자",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "Remember: Whenever you need help checking something, the student being evaluated",
			Replace:      "Remember: Whenever you need help checking something, the student being evaluated should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "Remember: Whenever you need help checking something, the student being evaluated",
			Replace:      "Remember: Whenever you need help checking something, the student being evaluated should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "The subject requests that a user with the login of the student being evaluated is present",
			Replace:      "The subject requests that a user with the login of the student being evaluated is present on the virtual machine. Check that it has been added and that it belongs to the \"sudo\" and \"user42\" groups.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "on the virtual machine. Check that it has been added and that it belongs to the",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "\"sudo\" and \"user42\" groups.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "Remember: Whenever you need help checking something, the student being evaluated",
			Replace:      "Remember: Whenever you need help checking something, the student being evaluated should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "This part is an opportunity to discuss the scores! The student being evaluated should",
			Replace:      "This part is an opportunity to discuss the scores! The student being evaluated should give you a brief explanation of how LVM works and what it is all about.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "give you a brief explanation of how LVM works and what it is all about.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "Remember: Whenever you need help checking something, the student being evaluated",
			Replace:      "Remember: Whenever you need help checking something, the student being evaluated should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "Remember: Whenever you need help checking something, the student being evaluated",
			Replace:      "Remember: Whenever you need help checking something, the student being evaluated should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "Remember: Whenever you need help checking something, the student being evaluated",
			Replace:      "Remember: Whenever you need help checking something, the student being evaluated should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "Remember: Whenever you need help checking something, the student being evaluated",
			Replace:      "Remember: Whenever you need help checking something, the student being evaluated should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "should be able to help you.",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "**Check, with the help of the subject and the student being evaluated, the bonus**",
			Replace:      "**Check, with the help of the subject and the student being evaluated, the bonus** **points authorized for this project:**",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "69c9d033-2ec0-44f5-a77c-7aab65e3a0ae",
			Remove:       "**points authorized for this project:**",
			Title:        "born2beroot - 채점표 (원본)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "70b04c69-a59e-45dd-aa59-884046ff28dd",
			Remove:       "그러나 이 값을 그대로 구하는 것은 불가능하므로, 확률변수 $X_{t-1}$의 값이",
			Replace:      "그러나 이 값을 그대로 구하는 것은 불가능하므로, 확률변수 $X_{t-1}$의 값이 x로 고정된 상황을 먼저 보자.",
			Title:        "6. 포아송과정의 응용",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "70b04c69-a59e-45dd-aa59-884046ff28dd",
			Remove:       "x로 고정된 상황을 먼저 보자.",
			Title:        "6. 포아송과정의 응용",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "79d7bd37-3128-49aa-b225-16b6e3e40229",
			Remove:       "무방향 그래프에서 적어도 한 개 이상의 경로로 연결된 정점들로 구성된",
			Replace:      "무방향 그래프에서 적어도 한 개 이상의 경로로 연결된 정점들로 구성된 종속 그래프를 연결 요소라고 한다.",
			Title:        "연결 요소",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "79d7bd37-3128-49aa-b225-16b6e3e40229",
			Remove:       "종속 그래프를 연결 요소라고 한다.",
			Title:        "연결 요소",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "8159b7e5-3ce9-4cfd-be5d-1d1a61a19ff8",
			Remove:       "준비한 데이터와 모델로 학습을 시작합니다. 학습 결과물(체크포인트)은",
			Replace:      "준비한 데이터와 모델로 학습을 시작합니다. 학습 결과물(체크포인트)은 미리 연동해둔 구글 드라이브의 준비된 위치 (`/gdrive/My Drive/nlpbook/checkpoint-doccls`)에 저장.",
			Title:        "4장 : 문서분류 모델",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "8159b7e5-3ce9-4cfd-be5d-1d1a61a19ff8",
			Remove:       "미리 연동해둔 구글 드라이브의 준비된 위치",
			Title:        "4장 : 문서분류 모델",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "8159b7e5-3ce9-4cfd-be5d-1d1a61a19ff8",
			Remove:       "(`/gdrive/My Drive/nlpbook/checkpoint-doccls`)에 저장.",
			Title:        "4장 : 문서분류 모델",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "8c25811a-fb8d-42b2-bfbd-3fca3da93bf7",
			Remove:       "다중 프로그래밍과 시간 공유의 목표를 동시에 달성하려면 프로세스의 일반적인 동작을",
			Replace:      "다중 프로그래밍과 시간 공유의 목표를 동시에 달성하려면 프로세스의 일반적인 동작을 고려해야 한다.",
			Title:        "프로세스의 스케쥴링과 연산",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "8c25811a-fb8d-42b2-bfbd-3fca3da93bf7",
			Remove:       "고려해야 한다.",
			Title:        "프로세스의 스케쥴링과 연산",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "8c25811a-fb8d-42b2-bfbd-3fca3da93bf7",
			Remove:       "일반적으로 대부분의 프로세스는 I/O 바운드 프로세스와 CPU 바운드 프로세스로 |",
			Replace:      "일반적으로 대부분의 프로세스는 I/O 바운드 프로세스와 CPU 바운드 프로세스로 | 나눌 수 있다.",
			Title:        "프로세스의 스케쥴링과 연산",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "8c25811a-fb8d-42b2-bfbd-3fca3da93bf7",
			Remove:       "나눌 수 있다.",
			Title:        "프로세스의 스케쥴링과 연산",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "a25e9212-5b73-4d14-98bc-87bea518bdce",
			Remove:       "이처럼 데이터 내에서 정답을 만들고, 학습하는 과정을 **자기지도 학습(self-supervised learning)**",
			Replace:      "이처럼 데이터 내에서 정답을 만들고, 학습하는 과정을 **자기지도 학습(self-supervised learning)** 이라고 한다.",
			Title:        "1장 : 자연어처리 개요",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "a25e9212-5b73-4d14-98bc-87bea518bdce",
			Remove:       "이라고 한다.",
			Title:        "1장 : 자연어처리 개요",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		// "연습문제와 시험"(a46c5b90-b06d-412b-b379-4179373c69c1)은 여기 안 넣는다.
		// 노션 원본은 스캔한 시험지 사진 세 장뿐이고, 지금 DB의 풀이 텍스트는
		// 나중에 admin에서 손으로 옮겨 쓴 것이다 — 변환 결과에 애초에 그 줄이
		// 없어서 표를 넣어도 재이관 때마다 "줄을 못 찾았다"로 계속 실패한다.
		// 서버 DB는 이미 SSH로 직접 합쳤다(2026-09-17).
		{
			NotionPageID: "bd664f34-3252-43d1-9883-ab4bd38d9eaf",
			Remove:       "나눠진 각 방을 어떻게 세팅할지 생각해볼 수 있듯이, 각 파티션을 세팅하는",
			Replace:      "나눠진 각 방을 어떻게 세팅할지 생각해볼 수 있듯이, 각 파티션을 세팅하는 방식도 다를 수 있다.",
			Title:        "born2beroot - OS: 배경지식",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "bd664f34-3252-43d1-9883-ab4bd38d9eaf",
			Remove:       "방식도 다를 수 있다.",
			Title:        "born2beroot - OS: 배경지식",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "c520daf4-6db6-44cd-9767-d0f0c560caa2",
			Remove:       "위의 4가지 조건만 만족하면 무조건 내적이라고 할 수 있기 때문에 다양한",
			Replace:      "위의 4가지 조건만 만족하면 무조건 내적이라고 할 수 있기 때문에 다양한 내적들이 나올 수 있다.",
			Title:        "8. 내적공간",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "c520daf4-6db6-44cd-9767-d0f0c560caa2",
			Remove:       "내적들이 나올 수 있다.",
			Title:        "8. 내적공간",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "cf83f154-e4b7-4911-85b6-fcc91b935ef1",
			Remove:       "이중 우선순위 큐(dual priority queue)는 전형적인 우선순위 큐처럼",
			Replace:      "이중 우선순위 큐(dual priority queue)는 전형적인 우선순위 큐처럼 데이터를 삽입, 삭제할 수 있는 자료 구조이다.",
			Title:        "7662. 이중 우선순위 큐",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "cf83f154-e4b7-4911-85b6-fcc91b935ef1",
			Remove:       "데이터를 삽입, 삭제할 수 있는 자료 구조이다.",
			Title:        "7662. 이중 우선순위 큐",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "cf83f154-e4b7-4911-85b6-fcc91b935ef1",
			Remove:       "전형적인 큐와의 차이점은 데이터를 삭제할 때 연산(operation) 명령에 따라",
			Replace:      "전형적인 큐와의 차이점은 데이터를 삭제할 때 연산(operation) 명령에 따라 우선순위가 가장 높은 데이터 또는 가장 낮은 데이터 중 하나를 삭제한다는 것이다.",
			Title:        "7662. 이중 우선순위 큐",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "cf83f154-e4b7-4911-85b6-fcc91b935ef1",
			Remove:       "우선순위가 가장 높은 데이터 또는 가장 낮은 데이터 중 하나를 삭제한다는 것이다.",
			Title:        "7662. 이중 우선순위 큐",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "dcd8e7c3-4d9c-4abf-b5b5-79103e163b18",
			Remove:       "   **그래서 서버에 문제가 생기면 이 복제물로 다시 작업을 시작할 수 있다.** 즉, 클라이언트 중에서 아무거나",
			Replace:      "   **그래서 서버에 문제가 생기면 이 복제물로 다시 작업을 시작할 수 있다.** 즉, 클라이언트 중에서 아무거나 골라도 서버를 복원할 수 있다.",
			Title:        "버전 관리 소프트웨어",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "dcd8e7c3-4d9c-4abf-b5b5-79103e163b18",
			Remove:       "   골라도 서버를 복원할 수 있다.",
			Title:        "버전 관리 소프트웨어",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "f46f9735-dea5-49ed-b2e6-dce62e0cebca",
			Remove:       "다른 클라이언트가 접속을 원하면",
			Replace:      "다른 클라이언트가 접속을 원하면 1024보단 크지만, 1625가 아닌 포트 번호를 부여하게 됨.",
			Title:        "프로세스 간 통신 (4) : 클라이언트 서버 환경",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "f46f9735-dea5-49ed-b2e6-dce62e0cebca",
			Remove:       "1024보단 크지만, 1625가 아닌 포트 번호를",
			Title:        "프로세스 간 통신 (4) : 클라이언트 서버 환경",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "f46f9735-dea5-49ed-b2e6-dce62e0cebca",
			Remove:       "부여하게 됨.",
			Title:        "프로세스 간 통신 (4) : 클라이언트 서버 환경",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "f69e826a-2112-4dbe-861d-8b61e0c136a5",
			Remove:       "만약 $a = \\left[ \\begin{matrix} 1 \\ 2 \\ 3 \\ 4 \\end{matrix} \\right]$인 경우,  ${\\mathbb{\\left|a\\right|} } = \\sqrt {1^2 + 2^2 + 3^2 + 4^2}$",
			Replace:      "만약 $a = \\left[ \\begin{matrix} 1 \\ 2 \\ 3 \\ 4 \\end{matrix} \\right]$인 경우,  ${\\mathbb{\\left|a\\right|} } = \\sqrt {1^2 + 2^2 + 3^2 + 4^2}$ 즉, 각 원소의 제곱을 다 더하고 거기에 근호를 취하면 된다.",
			Title:        "1. 벡터",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "f69e826a-2112-4dbe-861d-8b61e0c136a5",
			Remove:       "즉, 각 원소의 제곱을 다 더하고 거기에 근호를 취하면 된다.",
			Title:        "1. 벡터",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "f8503bcc-409f-4510-8e54-b68ee3f371b1",
			Remove:       "여기서 사용하는 salt 는 외부에서 사용자의 암호를 쉽게 파악하기 어렵도록 원본 데이터에 임의로 추가하는",
			Replace:      "여기서 사용하는 salt 는 외부에서 사용자의 암호를 쉽게 파악하기 어렵도록 원본 데이터에 임의로 추가하는 데이터이다.",
			Title:        "born2beroot - 보너스 (2)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "f8503bcc-409f-4510-8e54-b68ee3f371b1",
			Remove:       "데이터이다.",
			Title:        "born2beroot - 보너스 (2)",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "adfb3d58-9677-4b9d-93cb-932d029d036e",
			Remove:       "리버스 프록시 서버(엔진엑스)는 요청을 전달하고, 실제 요청에 대한 처리는",
			Replace:      "리버스 프록시 서버(엔진엑스)는 요청을 전달하고, 실제 요청에 대한 처리는 뒷단의 웹앱 서버가 처리",
			Title:        "10. 무중단 배포",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
		{
			NotionPageID: "adfb3d58-9677-4b9d-93cb-932d029d036e",
			Remove:       "뒷단의 웹앱 서버가 처리",
			Title:        "10. 무중단 배포",
			Why:          "문단을 줄바꿈으로 감싸 쓴 것을 한 줄로 합쳤다 (렌더러가 Enter 한 번으로도 <br>을 내게 되면서 안 원하는 줄바꿈이 보였다)",
		},
	}
}
