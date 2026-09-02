package curation

// `과제6`의 풀이를 마저 쓴다(2026-09-03).
//
// 원본은 답까지 갔다가 마지막에서 멈춰 있었다. 고칠 것이 셋이다.
//
//	정규화 상수  `1/(2π)`로 적혀 있다. 분산이 1인 정규분포라 `1/√(2π)`가 맞다
//	미분 부호    `∑(x_i-μ)·(-1) = ∑(x_i-μ)`로 좌우가 어긋난다
//	케이스       "$\mu$의 조건이 주어져 있으므로…"에서 끊기고 답만 있다
//
// **케이스를 나누는 근거가 이 문제의 핵심이다.** 제약 없는 최대점이 $\bar X$인데
// 모수는 $\mu \ge 0$이라, $\bar X < 0$이면 그 점을 쓸 수 없다. 로그가능도가
// 위로 볼록한 포물선이라 허용 구간에서는 단조감소가 되고, 그래서 최대는
// 경계 $\mu = 0$에서 난다. 그 한 줄이 빠져 있었다.
//
// 지시함수 $I(\mu>0)$이 로그 안에서 **합에 곱해져** 있던 것도 고친다. 그건
// 모수의 정의역을 말하는 것이라 항이 아니라 제약이다.
var mathstat1Homework6Edits = buildHomework6Edits()

func buildHomework6Edits() []BodyEdit {
	const page = "6bf4addc-22e6-4fba-8661-6f080b5079c6"
	var out []BodyEdit
	for _, line := range oldHomework6Lines {
		out = append(out, BodyEdit{
			NotionPageID: page, Remove: line,
			Title: "과제6",
			Why:   "풀이를 다시 쓴다. 옛 줄을 먼저 걷는다",
		})
	}
	out = append(out, BodyEdit{
		NotionPageID: page,
		Remove:       "2018111373 최인렬",
		Replace:      homework6Body,
		Title:        "과제6",
		Why:          "정규화 상수와 미분 부호를 고치고, 근거 없이 나오던 케이스 나누기를 채운다",
	})
	return out
}

// oldHomework6Lines는 걷어낼 옛 본문이다(첫 줄 제외). **차례가 곧 의미다.**
var oldHomework6Lines = []string{
	"## 가능도함수 구하기",
	"$$",
	"f_{X_i}(x_i;\\mu) = {1\\over2\\pi}~",
	"\\exp \\bigg( -{(x_i-\\mu)^2 \\over 2} \\bigg)~I(\\mu >0)",
	"$$",
	"$$",
	"\\begin{align*}",
	"&L(\\mu) = \\prod_{i=1}^{n}~",
	"f_{X_i}(x_i;\\mu)",
	"\\\\[20pt]",
	"&= \\bigg( {1\\over2\\pi} \\bigg)^n",
	"\\exp \\bigg( -{1 \\over 2}~ \\sum~",
	"(x_i-\\mu)^2\\bigg)~I(\\mu > 0)",
	"\\\\[30pt]",
	"&l(\\mu) = \\log(L(\\mu))",
	"\\\\[20pt]",
	"&= -{n \\over 2}~\\log(2\\pi)",
	"-{1 \\over 2}~\\sum_{i=1}^{n}",
	"~(x_i - \\mu)^2~I(\\mu > 0)",
	"\\end{align*}",
	"$$",
	"$$",
	"{\\partial \\over \\partial\\mu}~l(\\mu)",
	"= \\sum_{i=1}^{n}",
	"~(x_i - \\mu)\\cdot(-1) = \\sum_{i=1}^{n}",
	"~(x_i - \\mu) = 0",
	"\\\\[20pt]",
	"\\sum_{i=1}^{n}~x_i = n\\mu, \\kern{10pt}",
	"{1 \\over n}\\sum_{i=1}^{n}~x_i = \\overline{X} = \\mu",
	"$$",
	"그런데 $\\mu$의 조건이 주어져 있으므로…",
	"## 케이스 나누기",
	"1. $\\overline{X} < 0$ 의 경우 : $\\hat\\mu_{mle} = 0$",
	"2. $0 \\le \\overline{X}$의 경우 :  $\\hat\\mu_{mle} = \\overline{X}$",
}

// homework6Body는 새로 쓴 풀이다.
const homework6Body = `2018111373 최인렬

$X_1, \ldots, X_n \overset{iid}{\sim} N(\mu, 1)$이고 **모수공간이 $\mu \ge 0$으로 제한된** 경우의 최대가능도추정량을 구한다.

## 가능도함수 구하기

$$
f_{X_i}(x_i;\mu) = \dfrac{1}{\sqrt{2\pi}}
\exp \bigg( -\dfrac{(x_i-\mu)^2}{2} \bigg),
\qquad \mu \ge 0
$$

$$
\begin{align*}
L(\mu) &= \prod_{i=1}^{n} f_{X_i}(x_i;\mu)
\\[20pt]
&= \bigg( \dfrac{1}{\sqrt{2\pi}} \bigg)^{n}
\exp \bigg( -\dfrac{1}{2} \sum_{i=1}^{n} (x_i-\mu)^2 \bigg)
\\[30pt]
l(\mu) &= \log L(\mu)
\\[20pt]
&= -\dfrac{n}{2}\log(2\pi)
-\dfrac{1}{2} \sum_{i=1}^{n} (x_i - \mu)^2
\end{align*}
$$

$\mu \ge 0$은 로그 안의 항이 아니라 **정의역**이다. 지시함수를 합에 곱해 두면 $\log$를 씌울 때 $-\infty$가 되는 자리라 항처럼 다룰 수 없다.

$$
\begin{align*}
\dfrac{\partial}{\partial\mu} l(\mu)
&= -\dfrac{1}{2} \sum_{i=1}^{n} 2(x_i - \mu)\cdot(-1)
= \sum_{i=1}^{n} (x_i - \mu)
\\[20pt]
\dfrac{\partial^2}{\partial\mu^2} l(\mu) &= -n < 0
\end{align*}
$$

이계도함수가 어디서나 음수이므로 $l$은 **위로 볼록한 포물선**이고, 제약이 없다면 $\partial l/\partial\mu = 0$인 한 점에서 최대가 된다.

$$
\sum_{i=1}^{n} x_i = n\mu
\quad\Longrightarrow\quad
\mu = \dfrac{1}{n}\sum_{i=1}^{n} x_i = \bar X
$$

그런데 $\mu \ge 0$이라는 조건이 있으므로, $\bar X$가 그 구간 안에 있는지를 봐야 한다.

## 케이스 나누기

$l$이 위로 볼록하고 꼭짓점이 $\bar X$이므로, **꼭짓점의 오른쪽에서는 감소하고 왼쪽에서는 증가한다.**

**1. $\bar X \ge 0$인 경우.** 꼭짓점이 이미 허용 구간 안에 있다. 제약이 아무것도 막지 않으므로

$$
\hat\mu_{MLE} = \bar X
$$

**2. $\bar X < 0$인 경우.** 꼭짓점이 구간 밖에 있다. 허용 구간 $[0, \infty)$은 전부 꼭짓점의 오른쪽이라 그 위에서 $l$은 **단조감소**한다 — $\mu \ge 0$에서

$$
\dfrac{\partial}{\partial\mu} l(\mu) = n(\bar X - \mu) \le n\bar X < 0
$$

이므로 가장 왼쪽 끝에서 최대다.

$$
\hat\mu_{MLE} = 0
$$

**둘을 합치면** 최대가능도추정량은 $\bar X$를 0에서 자른 것이다.

$$
\hat\mu_{MLE} = \max(\bar X,\, 0) = \bar X^{+}
$$

미분해서 0이 되는 점을 찾는 것만으로는 이 답이 안 나온다. **최대는 정류점이 아니라 경계에서도 날 수 있고**, 모수공간이 열린 구간이 아닐 때는 언제나 끝을 함께 봐야 한다.`
