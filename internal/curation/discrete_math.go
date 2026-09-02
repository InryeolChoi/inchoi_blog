package curation

// 이산수학 글 셋의 본문 손질(2026-09-02).
//
// 카테고리 전체(10편)를 훑어 **비어 있는 절과 틀린 수식**을 찾았다. 나머지
// 일곱 편은 짧지만 내용이 맞아서 건드리지 않는다 — 짧은 것과 틀린 것은 다르다.
//
//	정수의 성질    `합동식` 절이 제목만 있다. 유클리드 호제법도 시작만 하고 끊긴다
//	분할          `p(n,2)`가 틀렸고, 절 둘이 제목만 있다
//	페르마의 소정리  법(modulus)이 `x`로 잘못 적혀 있고, 이항계수 위가 비어 있다
//
// **`수열과 유한급수`의 `# 수열`은 비어 있는 것이 아니다.** 바로 아래
// `## 등차수열`이 오는 상위 제목이라, 빈 절을 찾는 검사에 걸리지만 정상이다.
var discreteMathEdits = buildDiscreteMathEdits()

func buildDiscreteMathEdits() []BodyEdit {
	type step struct {
		pageID, title, remove, replace, why string
	}

	steps := []step{
		// ── 정수의 성질 ─────────────────────────────────────────
		{
			"022d0075-4bee-4724-9c39-71eb3e470952", "정수의 성질",
			"- 두 수의 최대공약수를 쉽게 알아내는 방법",
			euclidBody,
			"방법이라고만 하고 정작 그 방법(gcd(a,b) = gcd(b, a mod b))이 없었다",
		},
		{
			"022d0075-4bee-4724-9c39-71eb3e470952", "정수의 성질",
			"# 합동식", congruenceBody,
			"제목만 있고 내용이 없었다",
		},

		// ── 분할 ───────────────────────────────────────────────
		{
			"93dba7f0-6692-41a1-b1bf-4997f64d9938", "분할",
			`P(n,\; 2) = \frac{n}{2} \\[10pt]`,
			`p(n,\; 2) = \left\lfloor \frac{n}{2} \right\rfloor \\[10pt]`,
			"n을 두 자연수의 합으로 쓰는 방법은 n/2가 아니라 그 내림이다. n이 홀수면 n/2는 정수가 아니다",
		},
		{
			"93dba7f0-6692-41a1-b1bf-4997f64d9938", "분할",
			"## 동적 프로그래밍을 통한 분할", partitionDPBody,
			"제목만 있고 내용이 없었다",
		},
		{
			"93dba7f0-6692-41a1-b1bf-4997f64d9938", "분할",
			"# 집합의 분할", setPartitionBody,
			"제목만 있고 내용이 없었다",
		},

		// ── 페르마의 소정리 ──────────────────────────────────────
		// 법이 `x`로 적혀 있었다. 페르마의 소정리는 법이 소수 p일 때의
		// 정리라, x로 두면 정리 자체가 성립하지 않는다.
		{
			"772ab204-9061-424d-a215-b1bf05c72b98", "페르마의 소정리",
			`a^p \equiv a\pmod x~~\text{일때, }~~`,
			`a^p \equiv a\pmod p~~\text{이고, }~~`,
			"법이 x로 적혀 있었다. 소수 p가 맞다",
		},
		{
			"772ab204-9061-424d-a215-b1bf05c72b98", "페르마의 소정리",
			`a^p \bmod x = a \bmod x`,
			`a^p \bmod p = a \bmod p`,
			"법이 x로 적혀 있었다",
		},
		{
			"772ab204-9061-424d-a215-b1bf05c72b98", "페르마의 소정리",
			`a^{p-1} \bmod p = a^{1-1} \bmod x`,
			`a^{p-1} \bmod p = 1`,
			"우변이 a^0 mod x로 뒤엉켜 있었다. a와 p가 서로소일 때 값은 1이다",
		},
		{
			"772ab204-9061-424d-a215-b1bf05c72b98", "페르마의 소정리",
			`a^{p-n} \bmod p = a^{1-n} \bmod x`,
			fermatGeneralBody,
			"지수를 내리는 규칙이 뒤엉켜 있었다. 실제로 쓰는 꼴(모듈러 역원)로 바로잡는다",
		},
		{
			"772ab204-9061-424d-a215-b1bf05c72b98", "페르마의 소정리",
			`\binom{}{r} + \binom{}{r}`,
			`\binom{n-1}{r-1} + \binom{n-1}{r}`,
			"이항계수의 위 칸이 비어 있었다. 파스칼의 정리는 n-1이 들어가야 한다",
		},
		{
			"772ab204-9061-424d-a215-b1bf05c72b98", "페르마의 소정리",
			"##", "",
			"글자조차 없는 빈 제목이다",
		},
	}

	out := make([]BodyEdit, 0, len(steps))
	for _, s := range steps {
		out = append(out, BodyEdit{
			NotionPageID: s.pageID,
			Remove:       s.remove,
			Replace:      s.replace,
			Title:        s.title,
			Why:          s.why,
		})
	}
	return out
}

const euclidBody = `- 두 수의 최대공약수를 쉽게 알아내는 방법이다.

핵심은 다음 한 줄이다.

$$
\gcd(a,\ b) = \gcd(b,\ a \bmod b)
$$

$a = qb + r$이라 두면 $a$와 $b$의 공약수는 $r = a - qb$도 나누고, 거꾸로 $b$와 $r$의 공약수는 $a = qb + r$도 나눈다. 즉 두 쌍의 공약수 집합이 같으므로 최대공약수도 같다.

나머지는 매번 작아지고 음이 아니므로 언젠가 0이 된다. 그때의 나눈 수가 답이다.

$$
\gcd(a,\ 0) = a
$$

예를 들어 $\gcd(1071,\ 462)$는

$$
1071 = 2\cdot462 + 147,\quad 462 = 3\cdot147 + 21,\quad 147 = 7\cdot21 + 0
$$

이므로 $\gcd = 21$이다. 나머지가 적어도 절반씩 줄어들어 시간복잡도는 $O(\log \min(a,b))$다.

` + "```python" + `
def gcd(a, b):
    while b:
        a, b = b, a % b
    return a
` + "```" + `

최소공배수는 여기서 바로 나온다.

$$
\operatorname{lcm}(a,\ b) = \frac{a \cdot b}{\gcd(a,\ b)}
$$`

const congruenceBody = `# 합동식

$a$와 $b$를 $m$으로 나눈 나머지가 같을 때 **$m$을 법으로 합동**이라 하고 다음처럼 적는다.

$$
a \equiv b \pmod m \iff m \mid (a - b)
$$

나머지가 같다는 말과 차가 $m$의 배수라는 말이 같다는 것이 출발점이다.

**덧셈과 곱셈이 그대로 통한다.** $a \equiv b$, $c \equiv d \pmod m$이면

$$
a + c \equiv b + d,\qquad a - c \equiv b - d,\qquad ac \equiv bd \pmod m
$$

이다. 그래서 큰 수를 다룰 때 중간중간 나머지를 취해도 결과가 같다 — 이것이 프로그래밍에서 합동식을 쓰는 이유다.

**나눗셈은 그대로 통하지 않는다.** $ac \equiv bc \pmod m$이라고 해서 $a \equiv b$인 것은 아니다. 정확히는

$$
ac \equiv bc \pmod m \implies a \equiv b \!\!\pmod{\frac{m}{\gcd(c,\ m)}}
$$

이고, $c$와 $m$이 서로소일 때만 법이 그대로 남아 양변에서 $c$를 지울 수 있다. 예를 들어 $2\cdot3 \equiv 2\cdot0 \pmod 6$이지만 $3 \not\equiv 0 \pmod 6$이다.`

const fermatGeneralBody = `a^{p-1} \equiv 1 \pmod p \implies a^{k} \equiv a^{k \bmod (p-1)} \pmod p`

const partitionDPBody = `## 동적 프로그래밍을 통한 분할

$p(n, r)$에는 닫힌 공식이 없어서 점화식으로 구한다. $n$을 정확히 $r$개의 자연수로 나누는 방법을 **1을 쓰느냐 안 쓰느냐**로 가른다.

- 가장 작은 조각이 1이면, 그 1을 떼고 남은 $n-1$을 $r-1$개로 나눈다 → $p(n-1,\ r-1)$
- 모든 조각이 2 이상이면, 각 조각에서 1씩 빼도 여전히 자연수다 → $p(n-r,\ r)$

두 경우가 겹치지 않고 빠짐없으므로

$$
p(n,\ r) = p(n-1,\ r-1) + p(n-r,\ r)
$$

이다. 시작값은 $p(n,\ 1) = 1$, $p(n,\ n) = 1$이고, $r > n$이면 $0$이다.

` + "```python" + `
def partitions(n, r):
    # dp[i][j] = i를 정확히 j개의 자연수로 나누는 방법의 수
    dp = [[0] * (r + 1) for _ in range(n + 1)]
    dp[0][0] = 1
    for i in range(1, n + 1):
        for j in range(1, min(i, r) + 1):
            dp[i][j] = dp[i - 1][j - 1] + dp[i - j][j]
    return dp[n][r]
` + "```" + `

표를 한 번 채우므로 시간복잡도는 $O(nr)$이다. 분할수 $P_n$은 이 표의 한 행을 더하면 나온다.`

const setPartitionBody = `# 집합의 분할

앞의 분할은 **수**를 쪼갠 것이고, 여기서는 **원소를 구별하는 집합**을 쪼갠다. 같은 크기로 나누더라도 어떤 원소가 어디에 들어갔느냐가 다르면 다른 분할이다.

$n$개의 원소를 비어 있지 않은 $k$개의 묶음으로 나누는 방법의 수를 **제2종 스털링 수**라 하고 $S(n,\ k)$ 또는 $\left\{ {n \atop k} \right\}$로 적는다. 점화식은 수의 분할과 같은 방식으로 나온다 — $n$번째 원소를 **혼자 두느냐 아니냐**로 가른다.

- 혼자 두면 남은 $n-1$개를 $k-1$묶음으로 → $S(n-1,\ k-1)$
- 아니면 남은 $n-1$개를 이미 $k$묶음으로 나눈 뒤 그중 하나에 넣는다 → $k \cdot S(n-1,\ k)$

$$
S(n,\ k) = S(n-1,\ k-1) + k \cdot S(n-1,\ k)
$$

시작값은 $S(0,\ 0) = 1$이고 $S(n,\ 0) = 0\ (n \ge 1)$이다.

묶음 수를 정하지 않고 전부 더한 것이 **벨 수**다.

$$
B_n = \sum_{k=0}^{n} S(n,\ k)
$$

$B_0$부터 차례로 $1,\ 1,\ 2,\ 5,\ 15,\ 52,\ 203$이다. 예를 들어 $\{a, b, c\}$의 분할은 $\{abc\}$, $\{ab\}\{c\}$, $\{ac\}\{b\}$, $\{bc\}\{a\}$, $\{a\}\{b\}\{c\}$의 다섯 가지로 $B_3 = 5$와 맞는다.`
