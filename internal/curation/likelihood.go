package curation

// `분포별 가능도함수`를 다시 쓴다(2026-09-03).
//
// 원본에 **비어 있는 수식이 넷** 있었다. 노션 변환기가 `<!-- 원본에 비어있던
// 수식 -->` 주석으로 표시해 둔 자리다.
//
//	감마     곱과 로그가능도가 둘 다 비어 있음
//	지수     곱이 비어 있고, 밀도의 부호가 틀림(λe^{λx})
//	포아송   곱 자리에 단일 밀도가 그대로 복사돼 있음
//	라플라스  절 제목만 있고 아무것도 없음
//
// 훑는 김에 틀린 것도 고친다. 정규분포의 정규화 상수가 `1/√(2πσ)`로 적혀
// 있었고(σ²이 맞다), `θ = σ²` 절이 실제로는 σ로 미분하면서 이계도함수 부호가
// 뒤집혀 있었으며, 기하분포는 지수에 `n`을 써서 표본 크기와 겹쳤다.
//
// # 왜 줄 하나씩이 아니라 통째로 바꾸나
//
// 이 글은 `$$`가 스무 번, `\begin{align*}`이 다섯 번 나온다. `replaceLine`은
// 첫 번째 것만 바꾸므로 **줄을 집어 고칠 수가 없다.** 그래서 옛 줄을 문서
// 순서대로 전부 걷어낸 뒤 첫 줄 자리에 새 본문을 통째로 넣는다.
//
// **삭제가 먼저다.** 새 본문에도 `$$`가 있어서, 넣고 나서 지우면 방금 넣은
// 것이 지워진다 — 데이터베이스 표지에서 실제로 그렇게 절 하나를 잃었다.
var likelihoodEdits = buildLikelihoodEdits()

func buildLikelihoodEdits() []BodyEdit {
	const page = "df333507-a679-4beb-b165-285ae3bf42fc"
	var out []BodyEdit

	// 1) 옛 본문을 첫 줄만 남기고 전부 걷는다. **삭제가 먼저다** — 새 본문에
	//    `$$`처럼 같은 줄이 있어서, 넣고 나서 지우면 방금 넣은 것이 지워진다.
	for _, line := range oldLikelihoodLines {
		out = append(out, BodyEdit{
			NotionPageID: page, Remove: line,
			Title: "분포별 가능도함수",
			Why:   "절을 통째로 다시 쓴다. 옛 줄을 먼저 걷는다",
		})
	}

	// 2) 남은 첫 줄 자리에 새 본문을 통째로 넣는다.
	out = append(out, BodyEdit{
		NotionPageID: page,
		Remove:       "분포별 가능도함수를 정리해보자. (가능도함수 = 확률분포의 n제곱)",
		Replace:      likelihoodBody,
		Title:        "분포별 가능도함수",
		Why:          "감마·지수·라플라스의 빈 수식을 채우고, 포아송의 곱과 정규·기하의 틀린 식을 고친다",
	})
	return out
}

// oldLikelihoodLines는 걷어낼 옛 본문의 줄들이다(첫 줄 제외).
// **차례가 곧 의미다** — removeLine이 첫 번째 것만 지우므로 문서 순서대로 적는다.
var oldLikelihoodLines = []string{
	"## 정규",
	"$$",
	"\\begin{align*}",
	"f_{X_i}(x; \\theta) &= \\dfrac{1}{\\sqrt{2\\pi\\sigma}}",
	"\\exp \\bigg(- \\dfrac{(x-\\mu)^2}{2\\sigma^2} \\bigg)",
	"\\\\[20pt]",
	"\\prod_{i=1}^{ n}",
	"~f_{x_i}(x; \\theta) &=",
	"\\bigg({1\\over \\sqrt{2\\pi\\sigma}}\\bigg)^n",
	"~\\exp \\bigg(-{1 \\over 2\\sigma^2}~",
	"\\sum_{i=1}^{n}~",
	"\\big(X_i - \\mu\\big)^2",
	"\\bigg)",
	"\\end{align*}",
	"$$",
	"$\\theta = \\mu$인 경우",
	"$$",
	"\\begin{align*}",
	"l(\\theta) &=",
	"-{n \\over 2}~log(2\\pi\\sigma^2)",
	"-{1 \\over 2\\sigma^2}~\\sum_{i=1}^{n}",
	"\\big(X_i - \\mu\\big)^2",
	"\\\\[20pt]",
	"\\dfrac{d}{d\\theta}~l(\\theta) &=",
	"-{1 \\over \\sigma^2}~",
	"\\sum_{i=1}^{n}\\big(X_i - \\mu\\big)",
	"\\cdot (-1)",
	"= {1 \\over \\sigma^2}~",
	"\\big(n\\overline{X} - n\\mu\\big)",
	"\\\\[20pt]",
	"\\dfrac{d^2}{d^2\\theta}~l(\\theta) &=-{n \\over \\sigma^2}",
	"\\end{align*}",
	"$$",
	"$\\theta = \\sigma^2$인 경우",
	"$$",
	"\\begin{align*}",
	"l(\\theta) &=",
	"-{n \\over 2}~log(2\\pi\\sigma^2)",
	"-{1 \\over 2\\sigma^2}~\\sum_{i=1}^{n}",
	"\\big(X_i - \\mu\\big)^2",
	"\\\\[20pt]",
	"\\dfrac{d}{d\\theta}~l(\\theta)",
	"&=-{n \\over \\sigma}-",
	"{1 \\over 2}\\cdot\\bigg(",
	"-{2\\sigma\\over\\sigma^4}\\bigg)",
	"~\\sum_{i=1}^{n}",
	"\\big(X_i - \\mu\\big)^2",
	"\\\\[20pt]",
	"&= -{n \\over \\sigma}",
	"+ {1 \\over \\sigma^3}",
	"~\\sum_{i=1}^{n}",
	"\\big(X_i - \\mu\\big)^2",
	"\\\\[20pt]",
	"\\dfrac{d^2}{d^2\\theta}~l(\\theta) &= -{n \\over \\sigma^2}~",
	"+ {3 \\over \\sigma^4}",
	"~\\sum_{i=1}^{n}",
	"\\big(X_i - \\mu\\big)^2",
	"\\end{align*}",
	"$$",
	"## 기하",
	"$$",
	"\\begin{align*}",
	"f_X(x) &= p(1-p)^{n-1}",
	"\\\\[15pt]",
	"\\prod_{i=1}^{n}~f_{X_i}(x) &=",
	"p^n~(1-p)^{\\sum x_i -n}",
	"\\end{align*}",
	"$$",
	"## 감마",
	"$$",
	"\\begin{align*}",
	"f_{X_i}(x_i) &=",
	"{1\\over\\Gamma(\\alpha)~\\beta^\\alpha}",
	"~x^{\\alpha-1}",
	"~e^{-x/\\beta}",
	"\\\\[20pt]",
	"\\prod_{i=1}^{n}~f_{X_i}(x_i) &=",
	"\\end{align*}",
	"$$",
	"<!-- 원본에 비어있던 수식 -->",
	"$$",
	"l(\\theta) =",
	"$$",
	"## 지수",
	"$$",
	"f_{X_i}(x_i) = \\lambda e^{\\lambda x}",
	"$$",
	"$$",
	"\\prod_{i=1}^{n}~f_{X_i}(x_i) =",
	"$$",
	"## 포아송",
	"$$",
	"f_{X_i}(x_i) =",
	"{e^{-\\lambda}~\\lambda^x \\over x!}",
	"$$",
	"$$",
	"\\prod_{i=1}^{n}~f_{X_i}(x_i) =",
	"{e^{-\\lambda}~\\lambda^x \\over x!}",
	"$$",
	"## 이중지수 (라플라스)",
	"<!-- 원본에 비어있던 수식 -->",
}

// likelihoodBody는 새로 쓴 본문이다.
const likelihoodBody = `분포별 가능도함수를 정리해보자. 표본 $x_1, \ldots, x_n$이 서로 독립이므로 **가능도함수는 확률함수의 곱**이고, 로그를 씌우면 합이 되어 미분하기 쉬워진다.

$$
L(\theta) = \prod_{i=1}^{n} f_{X_i}(x_i; \theta),
\qquad
l(\theta) = \log L(\theta)
$$

## 정규

$$
\begin{align*}
f_{X_i}(x_i; \theta) &= \dfrac{1}{\sqrt{2\pi\sigma^2}}
\exp \bigg(- \dfrac{(x_i-\mu)^2}{2\sigma^2} \bigg)
\\[20pt]
L(\theta) &=
\bigg(\dfrac{1}{\sqrt{2\pi\sigma^2}}\bigg)^n
\exp \bigg(-\dfrac{1}{2\sigma^2}
\sum_{i=1}^{n} (x_i - \mu)^2 \bigg)
\\[20pt]
l(\theta) &=
-\dfrac{n}{2}\log(2\pi\sigma^2)
-\dfrac{1}{2\sigma^2}\sum_{i=1}^{n} (x_i - \mu)^2
\end{align*}
$$

로그가능도는 아래 두 경우가 함께 쓴다. 무엇으로 미분하느냐만 다르다.

**$\theta = \mu$인 경우**

$$
\begin{align*}
\dfrac{\partial}{\partial\mu} l &=
\dfrac{1}{\sigma^2} \sum_{i=1}^{n} (x_i - \mu)
= \dfrac{n}{\sigma^2}(\bar x - \mu)
\\[20pt]
\dfrac{\partial^2}{\partial\mu^2} l &= -\dfrac{n}{\sigma^2} < 0
\end{align*}
$$

이므로 $\hat\mu = \bar X$이고, 이계도함수가 음수라 최대점이다.

**$\theta = \sigma^2$인 경우.** $v = \sigma^2$으로 두고 **$v$로** 미분한다. $\sigma$로 미분해도 답은 같지만, 모수를 $\sigma^2$이라 해 놓고 $\sigma$로 미분하면 표기와 계산이 어긋난다.

$$
\begin{align*}
l(v) &= -\dfrac{n}{2}\log(2\pi v) - \dfrac{1}{2v}\sum_{i=1}^{n}(x_i-\mu)^2
\\[20pt]
\dfrac{d}{dv} l &= -\dfrac{n}{2v} + \dfrac{1}{2v^2}\sum_{i=1}^{n}(x_i-\mu)^2 = 0
\\[20pt]
\dfrac{d^2}{dv^2} l &= \dfrac{n}{2v^2} - \dfrac{1}{v^3}\sum_{i=1}^{n}(x_i-\mu)^2
\end{align*}
$$

첫 식을 풀면 $\hat\sigma^2 = \dfrac{1}{n}\sum_{i=1}^{n}(x_i-\mu)^2$이고, 그 값을 이계도함수에 넣으면 $\dfrac{n}{2\hat v^2} - \dfrac{n}{\hat v^2} = -\dfrac{n}{2\hat v^2} < 0$이라 최대점이다.

## 기하

$X$는 **첫 성공까지의 시행 횟수**다($x = 1, 2, \ldots$). 지수에 $n$을 쓰면 표본 크기와 겹치므로 $x$로 적는다.

$$
\begin{align*}
f_X(x) &= p(1-p)^{x-1}
\\[15pt]
L(p) &= p^n (1-p)^{\sum_{i=1}^{n} x_i - n}
\\[15pt]
l(p) &= n\log p + \bigg(\sum_{i=1}^{n} x_i - n\bigg)\log(1-p)
\\[15pt]
\dfrac{d}{dp} l &= \dfrac{n}{p} - \dfrac{\sum x_i - n}{1-p} = 0
\end{align*}
$$

정리하면 $n(1-p) = p\big(\sum x_i - n\big)$, 곧 $n = p\sum x_i$이므로

$$
\hat p = \dfrac{n}{\sum_{i=1}^{n} x_i} = \dfrac{1}{\bar X}
$$

이다. 기댓값이 $E[X] = 1/p$이니 적률법으로 푼 결과와 같다.

## 감마

모수는 형상 $\alpha$와 척도 $\beta$다($x > 0$).

$$
\begin{align*}
f_{X_i}(x_i) &=
\dfrac{1}{\Gamma(\alpha)\,\beta^\alpha}
\, x_i^{\alpha-1} e^{-x_i/\beta}
\\[20pt]
L(\alpha, \beta) &=
\bigg(\dfrac{1}{\Gamma(\alpha)\,\beta^\alpha}\bigg)^{n}
\bigg(\prod_{i=1}^{n} x_i\bigg)^{\alpha-1}
\exp\bigg(-\dfrac{1}{\beta}\sum_{i=1}^{n} x_i\bigg)
\\[20pt]
l(\alpha, \beta) &=
-n\log\Gamma(\alpha) - n\alpha\log\beta
+ (\alpha-1)\sum_{i=1}^{n}\log x_i
- \dfrac{1}{\beta}\sum_{i=1}^{n} x_i
\end{align*}
$$

**$\alpha$를 안다고 하면 $\beta$는 바로 풀린다.**

$$
\dfrac{\partial}{\partial\beta} l = -\dfrac{n\alpha}{\beta} + \dfrac{1}{\beta^2}\sum_{i=1}^{n} x_i = 0
\quad\Longrightarrow\quad
\hat\beta = \dfrac{\bar X}{\alpha}
$$

이계도함수는 $\dfrac{n\alpha}{\beta^2} - \dfrac{2}{\beta^3}\sum x_i$인데, $\hat\beta$를 넣으면 $-\dfrac{n\alpha}{\hat\beta^2} < 0$이라 최대점이다.

**$\alpha$까지 추정하려면 닫힌 해가 없다.**

$$
\dfrac{\partial}{\partial\alpha} l =
-n\,\psi(\alpha) - n\log\beta + \sum_{i=1}^{n}\log x_i = 0
$$

에 **디감마 함수** $\psi(\alpha) = \Gamma'(\alpha)/\Gamma(\alpha)$가 들어가서 손으로 못 푼다. 수치해석(뉴턴-랩슨 등)으로 구한다.

## 지수

$$
\begin{align*}
f_{X_i}(x_i) &= \lambda e^{-\lambda x_i}, \qquad x_i > 0
\\[20pt]
L(\lambda) &=
\lambda^n \exp\bigg(-\lambda \sum_{i=1}^{n} x_i\bigg)
\\[20pt]
l(\lambda) &= n\log\lambda - \lambda\sum_{i=1}^{n} x_i
\\[20pt]
\dfrac{d}{d\lambda} l &= \dfrac{n}{\lambda} - \sum_{i=1}^{n} x_i = 0
\\[20pt]
\dfrac{d^2}{d\lambda^2} l &= -\dfrac{n}{\lambda^2} < 0
\end{align*}
$$

$$
\hat\lambda = \dfrac{n}{\sum_{i=1}^{n} x_i} = \dfrac{1}{\bar X}
$$

지수분포는 **감마분포에서 $\alpha = 1$, $\beta = 1/\lambda$인 경우**다. 위 감마의 $\hat\beta = \bar X/\alpha$에 $\alpha=1$을 넣으면 $\hat\beta = \bar X$, 곧 $\hat\lambda = 1/\bar X$로 같은 답이 나온다.

## 포아송

$$
\begin{align*}
f_{X_i}(x_i) &=
\dfrac{e^{-\lambda}\lambda^{x_i}}{x_i!}, \qquad x_i = 0, 1, 2, \ldots
\\[20pt]
L(\lambda) &=
\dfrac{e^{-n\lambda}\,\lambda^{\sum_{i=1}^{n} x_i}}{\prod_{i=1}^{n} x_i!}
\\[20pt]
l(\lambda) &=
-n\lambda + \bigg(\sum_{i=1}^{n} x_i\bigg)\log\lambda
- \sum_{i=1}^{n}\log(x_i!)
\\[20pt]
\dfrac{d}{d\lambda} l &= -n + \dfrac{1}{\lambda}\sum_{i=1}^{n} x_i = 0
\\[20pt]
\dfrac{d^2}{d\lambda^2} l &= -\dfrac{1}{\lambda^2}\sum_{i=1}^{n} x_i < 0
\end{align*}
$$

$$
\hat\lambda = \dfrac{1}{n}\sum_{i=1}^{n} x_i = \bar X
$$

$\sum\log(x_i!)$은 $\lambda$와 무관하므로 미분에서 사라진다. 포아송은 평균과 분산이 둘 다 $\lambda$라, 적률법으로 풀어도 같은 답이다.

## 이중지수 (라플라스)

위치 $\mu$, 척도 $b > 0$이다. 정규분포와 달리 지수에 **제곱이 아니라 절댓값**이 들어간다.

$$
\begin{align*}
f_{X_i}(x_i) &=
\dfrac{1}{2b}\exp\bigg(-\dfrac{|x_i - \mu|}{b}\bigg)
\\[20pt]
L(\mu, b) &=
\bigg(\dfrac{1}{2b}\bigg)^{n}
\exp\bigg(-\dfrac{1}{b}\sum_{i=1}^{n}|x_i - \mu|\bigg)
\\[20pt]
l(\mu, b) &= -n\log(2b) - \dfrac{1}{b}\sum_{i=1}^{n}|x_i - \mu|
\end{align*}
$$

**$\mu$는 미분으로 풀 수 없다.** $|x_i - \mu|$가 $x_i$에서 꺾여 미분이 안 되기 때문이다. 대신 $l$을 최대로 만드는 것은 $\sum|x_i-\mu|$를 **최소**로 만드는 것과 같은데, 그 값은 표본중앙값에서 최소가 된다.

$$
\hat\mu = \text{표본중앙값}
$$

정규분포의 MLE가 표본평균인 것과 나란히 놓고 보면 뜻이 분명해진다 — **제곱을 최소화하면 평균, 절댓값을 최소화하면 중앙값**이다. 라플라스분포가 이상치에 덜 흔들리는 이유가 여기 있다.

$\mu$가 정해지면 $b$는 미분으로 풀린다.

$$
\dfrac{\partial}{\partial b} l = -\dfrac{n}{b} + \dfrac{1}{b^2}\sum_{i=1}^{n}|x_i - \hat\mu| = 0
\quad\Longrightarrow\quad
\hat b = \dfrac{1}{n}\sum_{i=1}^{n}|x_i - \hat\mu|
$$`
