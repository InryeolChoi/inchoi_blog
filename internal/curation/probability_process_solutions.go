package curation

// BodyAppend는 노션 원본 변환 뒤에 사람이 요청한 마크다운을 덧붙이는 예외다.
// DB 본문만 손으로 고치면 다음 import가 덮어쓰므로 notion_page_id를 키로 둔다.
type BodyAppend struct {
	NotionPageID string
	Marker       string
	Markdown     string
	Title        string
	Why          string
}

// probabilityProcessAppends는 확률과정론 `중간 & 기말` 글의 해답이다.
// **여러 글의 해답이 생기면서 표를 파일별로 나눴다**(2026-09-02) —
// 최종 `BodyAppends`는 body_appends.go가 이 묶음들을 합쳐서 만든다.
var probabilityProcessAppends = []BodyAppend{
	{
		NotionPageID: "eb9b0b6c-697e-4ebe-b8b5-34a44e59095f",
		Marker:       "## 중간고사 해답",
		Title:        "중간 & 기말",
		Why:          "확률과정론 중간고사와 기말고사의 전 문항 해답을 작성하기로 했다",
		Markdown: `## 중간고사 해답

<details>
<summary><strong>중간고사 해답 보기</strong></summary>

### 1. 포함배제 공식 증명

증명할 식은 다음과 같다.

$$
P(A \cup B)=P(A)+P(B)-P(A\cap B)
$$

$A\cup B$를 서로 겹치지 않는 세 부분으로 나누면

$$
A\cup B=(A\setminus B)\cup(B\setminus A)\cup(A\cap B)
$$

이다. 세 사건은 서로 배반이므로 확률의 가산성에 의해

$$
P(A\cup B)
=P(A\setminus B)+P(B\setminus A)+P(A\cap B)
$$

이다. 한편

$$
\begin{aligned}
P(A)&=P(A\setminus B)+P(A\cap B),\\
P(B)&=P(B\setminus A)+P(A\cap B)
\end{aligned}
$$

이므로 두 식을 더하면 $A\cap B$가 두 번 포함된다. 따라서 한 번을 빼 주면

$$
\boxed{P(A\cup B)=P(A)+P(B)-P(A\cap B)}
$$

를 얻는다.

### 2. 분포함수의 조건과 분산

주어진 함수는

$$
g(x)=x(1-x)I(0\le x<0.5)
+xI(0.5\le x<0.7)+kI(0.7\le x)
$$

이다. $g$가 누적분포함수라면 $x\to\infty$일 때 $g(x)\to1$이어야 한다. $x\ge0.7$에서는 $g(x)=k$이므로

$$
\boxed{k=1}
$$

이다. 이때 $g$는 감소하지 않고 오른쪽 연속이며, $-\infty$에서 0, $+\infty$에서 1로 수렴하므로 누적분포함수의 조건을 만족한다.

이 분포에는 연속적인 부분과 점질량이 함께 있다. 연속 부분의 밀도는

$$
f_X(x)=
\begin{cases}
1-2x, & 0<x<0.5,\\
1, & 0.5<x<0.7,\\
0, & \text{그 밖의 경우}
\end{cases}
$$

이고, 누적분포함수의 점프 크기로부터

$$
\begin{aligned}
P(X=0.5)&=g(0.5)-g(0.5-)=0.5-0.25=0.25,\\
P(X=0.7)&=g(0.7)-g(0.7-)=1-0.7=0.3
\end{aligned}
$$

을 얻는다.

따라서 기댓값은

$$
\begin{aligned}
E[X]
&=\int_0^{0.5}x(1-2x)\,dx
+0.5(0.25)
+\int_{0.5}^{0.7}x\,dx
+0.7(0.3)\\
&=\frac{1}{24}+\frac18+\frac{3}{25}+\frac{21}{100}
=\frac{149}{300}
\end{aligned}
$$

이다. 같은 방법으로

$$
\begin{aligned}
E[X^2]
&=\int_0^{0.5}x^2(1-2x)\,dx
+0.5^2(0.25)
+\int_{0.5}^{0.7}x^2\,dx
+0.7^2(0.3)\\
&=\frac{1}{96}+\frac{1}{16}+\frac{109}{1500}+\frac{147}{1000}
=\frac{3511}{12000}
\end{aligned}
$$

이다. 그러므로

$$
\boxed{
\operatorname{Var}(X)
=E[X^2]-E[X]^2
=\frac{8263}{180000}
\approx0.04591
}
$$

이다.

### 3. 포아송과정의 조건부 공분산

$E[X_7]=21$이므로 포아송과정의 발생률은

$$
7\lambda=21\quad\Longrightarrow\quad\lambda=3
$$

이다. $X_3=n$이 주어지면 $[0,3]$에서 발생한 $n$개의 사건 중 $[0,2]$에 들어가는 사건의 수는

$$
X_2\mid X_3=n\sim\operatorname{Binomial}\left(n,\frac23\right)
$$

이다. 또한 독립증분 성질에 의해

$$
X_4=X_3+(X_4-X_3)
$$

에서 미래 증분 $X_4-X_3$은 $X_2$와 $X_3$으로부터 독립이다. 따라서

$$
\begin{aligned}
\operatorname{Cov}(X_2,X_4\mid X_3)
&=\operatorname{Cov}\bigl(X_2,X_3+(X_4-X_3)\mid X_3\bigr)\\
&=0
\end{aligned}
$$

이고, 정답은

$$
\boxed{\operatorname{Cov}(X_2,X_4\mid X_3)=0}
$$

이다.

### 4. 도착 간격의 조건부 확률밀도함수

발생률이 1인 포아송과정의 도착 간격은 서로 독립이고

$$
Y_k\overset{\mathrm{iid}}\sim\operatorname{Exp}(1)
$$

이다. 따라서 $Y_1$을 어떤 값으로 조건화해도 $Y_2$와 $Y_3$의 분포는 변하지 않는다.

$S=Y_2+Y_3$이라 하면 $S$는 발생률이 1이고 모양모수가 2인 감마분포를 따른다. 합성곱으로 계산하면

$$
\begin{aligned}
f_{S\mid Y_1}(s\mid y_1)
&=\int_0^s e^{-u}e^{-(s-u)}\,du\\
&=se^{-s},\qquad s\ge0
\end{aligned}
$$

이다. 그러므로

$$
\boxed{
f_{Y_2+Y_3\mid Y_1}(s\mid y_1)
=se^{-s}I(s\ge0)
}
$$

이며, 결과는 주어진 $y_1$의 값과 무관하다.

</details>

## 기말고사 해답

<details>
<summary><strong>기말고사 해답 보기</strong></summary>

### 1. 복합 포아송과정

$E[X_t]=5t$이므로 $X_t$는 발생률 5인 포아송과정이다. 특히

$$
X_3\sim\operatorname{Poisson}(15),
\qquad
X_5\sim\operatorname{Poisson}(25)
$$

이다. 또 $Y_i$는 서로 독립이고

$$
E[Y_i]=5,\qquad \operatorname{Var}(Y_i)=100
$$

이다.

#### A. $Z_3$의 분산

$Z_3=\sum_{i=1}^{X_3}Y_i$에 전체분산법칙을 적용한다. $X_3=n$일 때

$$
E[Z_3\mid X_3=n]=5n,
\qquad
\operatorname{Var}(Z_3\mid X_3=n)=100n
$$

이므로

$$
\begin{aligned}
\operatorname{Var}(Z_3)
&=E[\operatorname{Var}(Z_3\mid X_3)]
+\operatorname{Var}(E[Z_3\mid X_3])\\
&=100E[X_3]+25\operatorname{Var}(X_3)\\
&=100(15)+25(15)\\
&=1875
\end{aligned}
$$

이다. 따라서

$$
\boxed{\operatorname{Var}(Z_3)=1875}
$$

이다.

#### B. $X_5$와 $Z_5$의 공분산

$E[Z_5\mid X_5]=5X_5$이고, $X_5$를 조건으로 두면 $X_5$는 상수이므로

$$
\begin{aligned}
\operatorname{Cov}(X_5,Z_5)
&=\operatorname{Cov}\bigl(X_5,E[Z_5\mid X_5]\bigr)\\
&=\operatorname{Cov}(X_5,5X_5)\\
&=5\operatorname{Var}(X_5)\\
&=5(25)=125
\end{aligned}
$$

이다. 따라서

$$
\boxed{\operatorname{Cov}(X_5,Z_5)=125}
$$

이다.

### 2. 마코프체인

추이확률행렬은

$$
P=
\begin{pmatrix}
0.2&0.4&0.4\\
0.5&0.5&0\\
0.6&0.4&0
\end{pmatrix}
$$

이다.

#### A. 상태 3의 주기

상태 3에서 출발해 2단계 뒤에 돌아오는 경로 $3\to1\to3$이 있고

$$
p_{31}p_{13}=0.6\times0.4>0
$$

이다. 또한 3단계 뒤에 돌아오는 경로 $3\to1\to1\to3$도 있고

$$
p_{31}p_{11}p_{13}=0.6\times0.2\times0.4>0
$$

이다. 가능한 귀환시간 집합에 2와 3이 모두 들어가므로

$$
d(3)=\gcd\{n\ge1:p_{33}^{(n)}>0\}=\gcd(2,3,\ldots)=1
$$

이다. 따라서

$$
\boxed{\text{상태 3의 주기는 }1}
$$

이며 상태 3은 비주기적이다.

#### B. $X_0$과 $X_1$의 공분산

초기확률함수는

$$
\phi=(P(X_0=1),P(X_0=2),P(X_0=3))
=\left(\frac14,\frac14,\frac12\right)
$$

이다. 먼저

$$
E[X_0]=1\left(\frac14\right)+2\left(\frac14\right)+3\left(\frac12\right)
=\frac94
$$

이다. 각 상태에서 다음 상태의 조건부 평균은

$$
\begin{aligned}
E[X_1\mid X_0=1]&=1(0.2)+2(0.4)+3(0.4)=2.2,\\
E[X_1\mid X_0=2]&=1(0.5)+2(0.5)=1.5,\\
E[X_1\mid X_0=3]&=1(0.6)+2(0.4)=1.4
\end{aligned}
$$

이다. 따라서

$$
E[X_1]
=\frac14(2.2)+\frac14(1.5)+\frac12(1.4)
=\frac{13}{8}
$$

이고

$$
\begin{aligned}
E[X_0X_1]
&=\sum_{i=1}^3P(X_0=i)\,i\,E[X_1\mid X_0=i]\\
&=\frac14(1)(2.2)+\frac14(2)(1.5)+\frac12(3)(1.4)\\
&=\frac{17}{5}
\end{aligned}
$$

이다. 그러므로

$$
\begin{aligned}
\operatorname{Cov}(X_0,X_1)
&=E[X_0X_1]-E[X_0]E[X_1]\\
&=\frac{17}{5}-\frac94\cdot\frac{13}{8}\\
&=\boxed{-\frac{41}{160}}
\end{aligned}
$$

이다.

#### C. 극한 추이확률과 $P(X_\infty=2)$

세 상태는 서로 도달할 수 있으므로 체인은 기약이다. 또한 상태 1과 상태 2에는 자기고리가 있으므로 체인은 비주기적이다. 따라서 $P^t$는 유일한 정상분포 $\pi$로 수렴한다.

$\pi P=\pi$와 $\pi_1+\pi_2+\pi_3=1$을 풀면

$$
\pi_3=0.4\pi_1,
\qquad
\pi_2=0.4\pi_1+0.5\pi_2+0.4\pi_3
$$

이므로

$$
\boxed{
\pi=\left(\frac{25}{63},\frac{28}{63},\frac{10}{63}\right)
=\left(\frac{25}{63},\frac49,\frac{10}{63}\right)
}
$$

이다. 따라서 초기 상태 $i$와 관계없이

$$
\boxed{
\lim_{t\to\infty}p_{ij}(t)=\pi_j
}
$$

이고, 행렬로 쓰면

$$
\lim_{t\to\infty}P^t
=
\begin{pmatrix}
25/63&28/63&10/63\\
25/63&28/63&10/63\\
25/63&28/63&10/63
\end{pmatrix}
$$

이다. 특히

$$
\boxed{P(X_\infty=2)=\pi_2=\frac49}
$$

이다.

</details>`,
	},
}
