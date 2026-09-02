package curation

// mathstat1ExamAppends는 수리통계1 `연습문제와 시험` 글에 붙일 해답이다.
// 본문에는 문제 이미지만 있고 풀이가 없어서, 네 절 각각에 해답을 덧붙인다.
var mathstat1ExamAppends = []BodyAppend{
	{
		NotionPageID: "a46c5b90-b06d-412b-b379-4179373c69c1",
		Marker:       "## 연습문제 1 해답",
		Title:        "연습문제와 시험",
		Why:          "수리통계1의 연습문제·중간·기말고사 전 문항 해답을 작성했다",
		Markdown: `## 연습문제 1 해답

<details>
<summary><strong>연습문제 1 해답 보기</strong></summary>

### 1. $X^2$의 분산

시험지에 인쇄된 식은 다음과 같다.

$$
F_X(s)=0.2\,I(s\ge 1)+0.4\,I\!\left(8\le (s-1)^3\right)+0.8\,I(2\le s<3)
$$

$8\le (s-1)^3$은 $s\ge 3$과 같은 말이므로, 인쇄된 계수를 그대로 쓰면

$$
F_X(s)=0.2\ (1\le s<2),\quad 0.2+0.8=1.0\ (2\le s<3),\quad 0.2+0.4=0.6\ (s\ge 3)
$$

가 되어 $s=3$에서 값이 **줄어든다.** 분포함수는 감소할 수 없으므로 이대로는 분포함수가 아니다.
두 계수 $0.4$와 $0.8$이 서로 바뀌어 인쇄된 것으로 읽으면

$$
F_X(s)=0.2\,I(s\ge 1)+0.4\,I(2\le s<3)+0.8\,I\!\left(8\le (s-1)^3\right)
$$

이고, 이때

$$
F_X(s)=\begin{cases}0,&s<1\\0.2,&1\le s<2\\0.6,&2\le s<3\\1,&s\ge 3\end{cases}
$$

로 감소하지 않고 오른쪽 연속이며 극한이 $0$과 $1$이라 분포함수의 조건을 만족한다.
**아래 풀이는 이 해석을 따른다.**

따라서 $X$는 다음 확률질량함수를 갖는 이산확률변수다.

$$
P(X=1)=0.2,\qquad P(X=2)=0.4,\qquad P(X=3)=0.4
$$

$X^2$는 값 $1,4,9$를 각각 확률 $0.2,\,0.4,\,0.4$로 갖는다. 그러므로

$$
E[X^2]=1(0.2)+4(0.4)+9(0.4)=0.2+1.6+3.6=5.4
$$

$$
E[X^4]=1(0.2)+16(0.4)+81(0.4)=0.2+6.4+32.4=39
$$

$$
\operatorname{Var}(X^2)=E[X^4]-\left(E[X^2]\right)^2=39-(5.4)^2=39-29.16
$$

$$
\boxed{\operatorname{Var}(X^2)=9.84=\frac{246}{25}}
$$

### 2. 5일 연속 첫 판매가 레몬티일 확률

$X$를 하루 중 레몬티를 처음 파는 시점 **이전에** 판 음료의 수라 하면 $X$는 기하분포를 따르고

$$
E[X]=\frac{1-p}{p}=5\ \Longrightarrow\ 1-p=5p\ \Longrightarrow\ \boxed{p=\frac16}
$$

이다. 여기서 $p$는 임의의 한 잔이 레몬티일 확률이다.
하루의 첫 판매 음료가 레몬티라는 것은 $X=0$과 같은 사건이므로

$$
P(\text{그날 첫 잔이 레몬티})=P(X=0)=p=\frac16
$$

**필요한 조건.** 이 계산이 성립하려면 (가) 한 잔 한 잔이 레몬티인지가 서로 독립이고,
(나) 그 확률 $p$가 날마다 같으며, (다) 5일이 서로 독립이어야 한다. 그러면

$$
P=\left(\frac16\right)^{5}=\frac{1}{7776}
$$

$$
\boxed{P=\frac{1}{7776}\approx 0.0001286}
$$

### 3. 역변환법으로 만든 기하분포 표본의 평균과 분산

문제 2와 같은 기하분포(성공 전 실패 횟수, 값이 $0,1,2,\dots$)를 쓴다. 기댓값이 7이므로

$$
\frac{1-p}{p}=7\ \Longrightarrow\ p=\frac18,\qquad P(X=k)=\left(\frac78\right)^{k}\frac18
$$

$$
F(k)=P(X\le k)=1-\left(\frac78\right)^{k+1}
$$

역변환법은 $u$에 대해 $F(k)\ge u$인 가장 작은 정수 $k$를 고르는 것이다. 즉

$$
k=\left\lceil \frac{\ln(1-u)}{\ln(7/8)}\right\rceil-1,\qquad \ln(7/8)=-0.133531
$$

표본 $u=0.7,\,0.1,\,0.3,\,0.6$에 차례로 넣으면

$$
\frac{\ln 0.3}{\ln(7/8)}=9.016,\quad \frac{\ln 0.9}{\ln(7/8)}=0.789,\quad \frac{\ln 0.7}{\ln(7/8)}=2.671,\quad \frac{\ln 0.4}{\ln(7/8)}=6.862
$$

$$
x_1=9,\qquad x_2=0,\qquad x_3=2,\qquad x_4=6
$$

검산해 보면 $F(8)=0.6996<0.7\le F(9)=0.7371$, $F(0)=0.125\ge 0.1$,
$F(1)=0.2344<0.3\le F(2)=0.3301$, $F(5)=0.5512<0.6\le F(6)=0.6073$으로 모두 맞다.

평균은

$$
\bar x=\frac{9+0+2+6}{4}=\frac{17}{4}=4.25
$$

편차제곱합은

$$
\sum_{i=1}^{4}(x_i-\bar x)^2=(4.75)^2+(-4.25)^2+(-2.25)^2+(1.75)^2=22.5625+18.0625+5.0625+3.0625=48.75
$$

$$
s^2=\frac13\sum_{i=1}^{4}(x_i-\bar x)^2=\frac{48.75}{3}=16.25
$$

$$
\boxed{x=(9,0,2,6),\qquad \bar x=4.25,\qquad s^2=16.25}
$$

> 만약 기하분포를 값이 $1,2,\dots$인 "시행 횟수" 형태로 잡으면 $p=1/7$이고
> 표본은 $8,1,3,6$, $\bar x=4.5$, $s^2=29/3\approx 9.667$이 된다.
> 여기서는 2번 문제와 같은 정의를 쓰는 쪽을 택했다.

### 4. 동국전자의 두 공장

$A,B$를 각각 A공장·B공장에서 생산했다는 사건이라 하면 $P(A)=0.75$, $P(B)=0.25$이고

$$
P(\text{휴대폰 불량}\mid A)=\frac17,\quad P(\text{공유기 불량}\mid A)=\frac16
$$

$$
P(\text{휴대폰 불량}\mid B)=\frac18,\quad P(\text{공유기 불량}\mid B)=\frac19
$$

이다. 한 공장에서 나온 휴대폰 한 대와 공유기 한 대를 본다고 해석한다.

**(1) 휴대폰이나 공유기가 불량일 확률.** 같은 공장 안에서 두 불량이 독립이므로 여사건으로 계산하면

$$
P(\text{불량 있음}\mid A)=1-\left(1-\frac17\right)\left(1-\frac16\right)=1-\frac67\cdot\frac56=1-\frac57=\frac27
$$

$$
P(\text{불량 있음}\mid B)=1-\left(1-\frac18\right)\left(1-\frac19\right)=1-\frac78\cdot\frac89=1-\frac79=\frac29
$$

전확률 공식으로

$$
P(\text{불량 있음})=\frac34\cdot\frac27+\frac14\cdot\frac29=\frac{3}{14}+\frac{1}{18}=\frac{27}{126}+\frac{7}{126}=\frac{34}{126}
$$

$$
\boxed{P(\text{불량 있음})=\frac{17}{63}\approx 0.2698}
$$

**(2) 불량 휴대폰이 B공장 제품일 확률.** 먼저 휴대폰이 불량일 확률은

$$
P(\text{휴대폰 불량})=\frac34\cdot\frac17+\frac14\cdot\frac18=\frac{3}{28}+\frac{1}{32}=\frac{24}{224}+\frac{7}{224}=\frac{31}{224}
$$

베이즈 정리에 의해

$$
P(B\mid \text{휴대폰 불량})=\frac{\frac14\cdot\frac18}{\frac{31}{224}}=\frac{\frac{7}{224}}{\frac{31}{224}}
$$

$$
\boxed{P(B\mid \text{휴대폰 불량})=\frac{7}{31}\approx 0.2258}
$$

사전확률 $P(B)=0.25$보다 조금 낮아졌다. B공장의 불량률이 A공장보다 낮으므로,
불량이라는 정보는 A공장 쪽을 약간 더 의심하게 만든다 — 방향이 맞다.

</details>`,
	},
	{
		NotionPageID: "a46c5b90-b06d-412b-b379-4179373c69c1",
		Marker:       "## 연습문제 2 해답",
		Title:        "연습문제와 시험",
		Why:          "수리통계1의 연습문제·중간·기말고사 전 문항 해답을 작성했다",
		Markdown: `## 연습문제 2 해답

<details>
<summary><strong>연습문제 2 해답 보기</strong></summary>

영문 시험지(MTH 5411 Math Stat I, Fall 1999 Midterm)다. 문항을 옮기고 풀었다.

### 1. 세 동전 중 앞면이 나온 동전이 양면 동전일 확률 (20점)

상자에 동전이 셋 있다. 하나는 양면이 앞면인 동전, 하나는 공정한 동전,
나머지는 25%로 앞면이 나오는 편향 동전이다. 하나를 무작위로 골라 던졌더니 앞면이 나왔다.

$C_1,C_2,C_3$을 각각 양면·공정·편향 동전을 골랐다는 사건이라 하면 $P(C_i)=1/3$이고

$$
P(H\mid C_1)=1,\qquad P(H\mid C_2)=\frac12,\qquad P(H\mid C_3)=\frac14
$$

$$
P(H)=\frac13\left(1+\frac12+\frac14\right)=\frac13\cdot\frac74=\frac{7}{12}
$$

$$
P(C_1\mid H)=\frac{\frac13\cdot 1}{\frac{7}{12}}=\frac{\frac13}{\frac{7}{12}}
$$

$$
\boxed{P(C_1\mid H)=\frac47\approx 0.5714}
$$

### 2. 이산확률변수의 분포함수와 확률들 (20점)

$X$는 $\{-2,0,1,4\}$를 각각 확률 $0.3,\,0.2,\,0.4,\,0.1$로 갖는다.

**a) 분포함수 $F$.** 계단함수이고 각 값에서 그 확률만큼 뛴다.

$$
F_X(x)=\begin{cases}0,&x<-2\\0.3,&-2\le x<0\\0.5,&0\le x<1\\0.9,&1\le x<4\\1,&x\ge 4\end{cases}
$$

그림은 $x=-2,0,1,4$에서 각각 $0.3,\,0.2,\,0.4,\,0.1$만큼 수직으로 뛰고 그 사이는 수평인 계단이다.
각 계단은 왼쪽 끝점을 포함한다(오른쪽 연속).

**b) 각 확률.**

$$
P(X\ge -3)=1
$$

$$
P(X>1)=P(X=4)=0.1
$$

$$
P(X<0)=P(X=-2)=0.3
$$

$$
P(X=2)=0
$$

$$
P(1<X<4)=0
$$

$$
P(-1\le X<0)=0
$$

마지막 셋은 그 구간 안에 $X$가 가질 수 있는 값이 하나도 없어서 0이다.

$$
\boxed{1,\quad 0.1,\quad 0.3,\quad 0,\quad 0,\quad 0}
$$

### 3. 100세 이상 인구 (20점)

5년 관측 기준으로 평균 1.5명이므로 $N\sim \text{Poisson}(\lambda=1.5)$로 본다.

**a) 한 명보다 많을 확률.**

$$
P(N>1)=1-P(N=0)-P(N=1)=1-e^{-1.5}-1.5e^{-1.5}=1-2.5e^{-1.5}
$$

$e^{-1.5}=0.223130$이므로

$$
\boxed{P(N>1)=1-2.5(0.223130)=0.4422}
$$

**b) 아무도 없을 확률.**

$$
\boxed{P(N=0)=e^{-1.5}\approx 0.2231}
$$

### 4. 적률생성함수로 구하는 포아송의 평균과 분산 (20점)

$X\sim \text{Poisson}(\lambda)$의 적률생성함수는

$$
M_X(t)=E\!\left[e^{tX}\right]=\sum_{k=0}^{\infty}e^{tk}\frac{e^{-\lambda}\lambda^{k}}{k!}=e^{-\lambda}\sum_{k=0}^{\infty}\frac{(\lambda e^{t})^{k}}{k!}=e^{-\lambda}e^{\lambda e^{t}}
$$

$$
M_X(t)=\exp\!\left(\lambda\left(e^{t}-1\right)\right)
$$

미분하면

$$
M_X'(t)=\lambda e^{t}\exp\!\left(\lambda(e^{t}-1)\right),\qquad M_X'(0)=\lambda
$$

$$
M_X''(t)=\left(\lambda e^{t}+\lambda^{2}e^{2t}\right)\exp\!\left(\lambda(e^{t}-1)\right),\qquad M_X''(0)=\lambda+\lambda^{2}
$$

따라서

$$
E[X]=\lambda,\qquad \operatorname{Var}(X)=M_X''(0)-\left(M_X'(0)\right)^2=\lambda+\lambda^2-\lambda^2
$$

$$
\boxed{E[X]=\lambda,\qquad \operatorname{Var}(X)=\lambda}
$$

### 5. 정규확률변수의 일차변환 (20점)

$N_{\mu,\sigma^2}$의 적률생성함수는 $M(t)=\exp\!\left(\mu t+\frac{\sigma^2t^2}{2}\right)$이다.
$W=aN_{\mu,\sigma^2}+b$ ($a>0$)라 하면

**i) 적률생성함수.**

$$
M_W(t)=E\!\left[e^{t(aN+b)}\right]=e^{bt}M(at)=e^{bt}\exp\!\left(\mu at+\frac{\sigma^2a^2t^2}{2}\right)
$$

$$
\boxed{M_W(t)=\exp\!\left((a\mu+b)t+\frac{a^{2}\sigma^{2}t^{2}}{2}\right)}
$$

**ii) 평균과 분산.** 위 식은 평균 $a\mu+b$, 분산 $a^2\sigma^2$인 정규분포의 적률생성함수와 같은 꼴이다.
적률생성함수가 분포를 유일하게 정하므로

$$
W\sim N\!\left(a\mu+b,\ a^{2}\sigma^{2}\right)
$$

$$
\boxed{E[W]=a\mu+b,\qquad \operatorname{Var}(W)=a^{2}\sigma^{2}}
$$

</details>`,
	},
	{
		NotionPageID: "a46c5b90-b06d-412b-b379-4179373c69c1",
		Marker:       "## 중간고사 해답",
		Title:        "연습문제와 시험",
		Why:          "수리통계1의 연습문제·중간·기말고사 전 문항 해답을 작성했다",
		Markdown: `## 중간고사 해답

<details>
<summary><strong>중간고사 해답 보기</strong></summary>

### 1. 두 주사위의 합이 9보다 클 때까지 던진 횟수 (25점)

한 번 던져 두 눈의 합이 9보다 클 확률은 합이 10, 11, 12인 경우다.

$$
P(\text{합}>9)=\frac{3+2+1}{36}=\frac{6}{36}=\frac16
$$

던진 횟수 $N$은 **첫 성공이 나온 시행 번호**이므로 $N\sim \text{Geometric}(p=1/6)$, 값은 $1,2,\dots$이다.

$$
E[N]=\frac1p=6
$$

$$
\operatorname{Var}(N)=\frac{1-p}{p^{2}}=\frac{5/6}{1/36}=30
$$

$$
\boxed{E[N]=6,\qquad \operatorname{Var}(N)=30}
$$

### 2. $X^2+8X>9$일 확률 (25점)

$X$는 기댓값이 10인 지수분포이므로 $\lambda=1/10$이고 $P(X>x)=e^{-x/10}$이다.

$$
X^2+8X-9>0\ \Longleftrightarrow\ (X+9)(X-1)>0
$$

$X>0$이라 $X+9>0$은 항상 참이므로 이 부등식은 $X>1$과 같다.

$$
P(X>1)=e^{-1/10}
$$

$$
\boxed{P\!\left(X^2+8X>9\right)=e^{-0.1}\approx 0.9048}
$$

### 3. 혼합분포의 분산 (25점)

$$
F_X(x)=\frac{1}{12}I(x\ge 3)+\frac14 I(x\ge 7)+\frac{1}{20}(x-7)I(8\le x<9)+\frac23 I(x\ge 9)
$$

구간별로 값을 적어 보면

$$
F_X(x)=\begin{cases}0,&x<3\\ \frac{1}{12},&3\le x<7\\ \frac13,&7\le x<8\\ \frac13+\frac{x-7}{20},&8\le x<9\\ 1,&x\ge 9\end{cases}
$$

이다. 감소하지 않고 $x\ge 9$에서 $\frac{1}{12}+\frac14+\frac23=1$이므로 분포함수가 맞다.
점질량과 연속 부분이 섞인 혼합분포이고, 각 뛰는 자리를 읽으면

$$
P(X=3)=\frac1{12},\quad P(X=7)=\frac14,\quad P(X=8)=\frac1{20},\quad P(X=9)=1-\frac{13}{30}=\frac{17}{30}
$$

이며 $8<x<9$에서 밀도가 $f(x)=\frac{1}{20}$이다. ($x=8$의 점질량은 그 항이 $x=8$에서 $\frac{1}{20}$부터 시작하기 때문에 생긴다.)
질량을 모두 더하면

$$
\frac1{12}+\frac14+\frac1{20}+\int_{8}^{9}\frac{dx}{20}+\frac{17}{30}=\frac{20}{60}+\frac{3}{60}+\frac{3}{60}+\frac{34}{60}=1
$$

로 맞다. 기댓값은

$$
E[X]=3\cdot\frac1{12}+7\cdot\frac14+8\cdot\frac1{20}+\int_{8}^{9}\frac{x}{20}dx+9\cdot\frac{17}{30}
$$

$$
\int_{8}^{9}\frac{x}{20}dx=\frac{81-64}{40}=\frac{17}{40}
$$

$$
E[X]=\frac14+\frac74+\frac25+\frac{17}{40}+\frac{51}{10}=\frac{10+70+16+17+204}{40}=\frac{317}{40}=7.925
$$

이차적률은

$$
E[X^2]=9\cdot\frac1{12}+49\cdot\frac14+64\cdot\frac1{20}+\int_{8}^{9}\frac{x^{2}}{20}dx+81\cdot\frac{17}{30}
$$

$$
\int_{8}^{9}\frac{x^{2}}{20}dx=\frac{729-512}{60}=\frac{217}{60}
$$

$$
E[X^2]=\frac{45+735+192+217+2754}{60}=\frac{3943}{60}\approx 65.7167
$$

따라서

$$
\operatorname{Var}(X)=\frac{3943}{60}-\left(\frac{317}{40}\right)^{2}=\frac{315440}{4800}-\frac{301467}{4800}=\frac{13973}{4800}
$$

$$
\boxed{\operatorname{Var}(X)=\frac{13973}{4800}\approx 2.9110}
$$

### 4. $Y=X^2$의 확률밀도함수 (25점)

$$
f_X(x)=\frac{1}{36}\left(3x^{2}+2x\right)I(0\le x<k)
$$

먼저 $k$를 정한다. 전체 적분이 1이어야 하므로

$$
\int_{0}^{k}\frac{3x^{2}+2x}{36}dx=\frac{k^{3}+k^{2}}{36}=1\ \Longrightarrow\ k^{3}+k^{2}=36\ \Longrightarrow\ k=3
$$

($27+9=36$이다.) 그러면 $X\in[0,3)$이고 $Y=X^{2}\in[0,9)$이다.
$y\mapsto \sqrt y$가 $[0,9)$에서 일대일 증가이므로 변수변환 공식을 쓴다.

$$
f_Y(y)=f_X\!\left(\sqrt y\right)\left|\frac{d}{dy}\sqrt y\right|=\frac{3y+2\sqrt y}{36}\cdot\frac{1}{2\sqrt y}
$$

$$
f_Y(y)=\frac{3\sqrt y+2}{72},\qquad 0<y<9
$$

검산하면

$$
\int_{0}^{9}\frac{3\sqrt y+2}{72}dy=\frac{1}{72}\left[2y^{3/2}+2y\right]_{0}^{9}=\frac{54+18}{72}=1
$$

$$
\boxed{f_Y(y)=\frac{3\sqrt y+2}{72}\,I(0<y<9)}
$$

</details>`,
	},
	{
		NotionPageID: "a46c5b90-b06d-412b-b379-4179373c69c1",
		Marker:       "## 기말고사 해답",
		Title:        "연습문제와 시험",
		Why:          "수리통계1의 연습문제·중간·기말고사 전 문항 해답을 작성했다",
		Markdown: `## 기말고사 해답

<details>
<summary><strong>기말고사 해답 보기</strong></summary>

### 1. 주사위 눈의 합과 용돈의 상관계수 (30점)

주사위를 두 번 던진 눈의 합을 $S$라 하고, 다음 달 용돈 $Y$는 평균이 $S$이고 분산이 1인
정규분포에서 뽑은 값이다. 즉 $Z\sim N(0,1)$이 $S$와 독립일 때

$$
Y=S+Z
$$

로 쓸 수 있다. 주사위 한 개의 분산은

$$
\operatorname{Var}(D)=E[D^{2}]-\left(E[D]\right)^{2}=\frac{91}{6}-\left(\frac72\right)^{2}=\frac{182-147}{12}=\frac{35}{12}
$$

이고 두 번은 독립이므로

$$
\operatorname{Var}(S)=2\cdot\frac{35}{12}=\frac{35}{6}
$$

이다. 공분산과 분산은

$$
\operatorname{Cov}(S,Y)=\operatorname{Cov}(S,S+Z)=\operatorname{Var}(S)+\operatorname{Cov}(S,Z)=\frac{35}{6}
$$

$$
\operatorname{Var}(Y)=\operatorname{Var}(S)+\operatorname{Var}(Z)=\frac{35}{6}+1=\frac{41}{6}
$$

따라서

$$
\rho=\frac{\operatorname{Cov}(S,Y)}{\sqrt{\operatorname{Var}(S)\operatorname{Var}(Y)}}=\frac{35/6}{\sqrt{\frac{35}{6}\cdot\frac{41}{6}}}=\sqrt{\frac{35}{41}}
$$

$$
\boxed{\rho=\sqrt{\frac{35}{41}}\approx 0.9239}
$$

용돈에 얹히는 잡음의 분산 1이 $\operatorname{Var}(S)=35/6\approx 5.83$에 비해 작아서 상관이 1에 가깝다.

### 2. $3(X-Y)^2/(X+Y)^2$의 분포 (30점)

$X,Y$는 평균이 0인 이변량정규분포를 따르고 분산이 $\sigma^2$로 같으며 상관계수는 $0.5$다.

$$
U=X-Y,\qquad V=X+Y
$$

로 두면 $(U,V)$도 이변량정규분포이고 평균은 0이다.

$$
\operatorname{Var}(U)=2\sigma^{2}(1-\rho)=2\sigma^{2}\cdot\frac12=\sigma^{2}
$$

$$
\operatorname{Var}(V)=2\sigma^{2}(1+\rho)=2\sigma^{2}\cdot\frac32=3\sigma^{2}
$$

$$
\operatorname{Cov}(U,V)=\operatorname{Var}(X)-\operatorname{Var}(Y)=0
$$

**정규분포에서는 공분산 0이 곧 독립**이므로 $U$와 $V$는 독립이다. 따라서

$$
A=\left(\frac{U}{\sigma}\right)^{2}\sim \chi^{2}_{1},\qquad B=\left(\frac{V}{\sqrt3\,\sigma}\right)^{2}\sim \chi^{2}_{1}
$$

이고 둘은 독립이다. 구하는 양을 이 둘로 쓰면

$$
\frac{3(X-Y)^{2}}{(X+Y)^{2}}=\frac{3U^{2}}{V^{2}}=\frac{3\sigma^{2}A}{3\sigma^{2}B}=\frac{A}{B}=\frac{A/1}{B/1}
$$

자유도 1인 독립 카이제곱을 각자 자유도로 나눈 비이므로

$$
\boxed{\frac{3(X-Y)^{2}}{(X+Y)^{2}}\sim F(1,1)}
$$

이다. $\sigma$가 약분되어 분산 값과 무관하다. 밀도는

$$
f(w)=\frac{1}{\pi\sqrt w\,(1+w)},\qquad w>0
$$

이고, 같은 말로 표준 코시(자유도 1인 $t$) 확률변수의 제곱이다.

### 3. 지수분포 표본 3개의 최댓값과 최솟값의 곱 (40점)

기댓값이 1인 지수분포에서 $X_1,X_2,X_3$을 뽑고 순서통계량을
$X_{(1)}\le X_{(2)}\le X_{(3)}$이라 하자. $E\left[X_{(1)}X_{(3)}\right]$을 구한다.

**방법 1 — 지수분포의 무기억성(레니 표현).** $E_1,E_2,E_3$이 독립인 $\text{Exp}(1)$일 때

$$
X_{(1)}=\frac{E_1}{3},\qquad X_{(2)}=\frac{E_1}{3}+\frac{E_2}{2},\qquad X_{(3)}=\frac{E_1}{3}+\frac{E_2}{2}+E_3
$$

가 성립한다. 그러면

$$
E\!\left[X_{(1)}X_{(3)}\right]=E\!\left[\frac{E_1}{3}\left(\frac{E_1}{3}+\frac{E_2}{2}+E_3\right)\right]=\frac{E[E_1^{2}]}{9}+\frac{E[E_1]E[E_2]}{6}+\frac{E[E_1]E[E_3]}{3}
$$

$E[E_i]=1$, $E[E_i^{2}]=2$이므로

$$
E\!\left[X_{(1)}X_{(3)}\right]=\frac29+\frac16+\frac13=\frac{4}{18}+\frac{3}{18}+\frac{6}{18}=\frac{13}{18}
$$

**방법 2 — 결합밀도로 직접 적분(검산).** 최솟값 $U$와 최댓값 $V$의 결합밀도는

$$
f(u,v)=n(n-1)\left[F(v)-F(u)\right]^{n-2}f(u)f(v)=6\left(e^{-u}-e^{-v}\right)e^{-u}e^{-v},\quad 0<u<v
$$

이다. 안쪽 적분을 먼저 하면

$$
\int_{u}^{\infty}v e^{-v}dv=(u+1)e^{-u},\qquad \int_{u}^{\infty}v e^{-2v}dv=e^{-2u}\left(\frac{u}{2}+\frac14\right)
$$

이므로

$$
E[UV]=6\int_{0}^{\infty}\left(u^{2}+u\right)e^{-3u}du-6\int_{0}^{\infty}\left(\frac{u^{2}}{2}+\frac{u}{4}\right)e^{-3u}du
$$

$$
=6\left(\frac{2}{27}+\frac19\right)-6\left(\frac{1}{27}+\frac{1}{36}\right)=\frac{10}{9}-\frac{7}{18}=\frac{13}{18}
$$

두 방법의 값이 같다.

$$
\boxed{E\!\left[X_{(1)}X_{(3)}\right]=\frac{13}{18}\approx 0.7222}
$$

참고로 $E\left[X_{(1)}\right]=1/3$, $E\left[X_{(3)}\right]=1+\frac12+\frac13=\frac{11}{6}$이라
독립이었다면 $\frac{11}{18}$이었을 것이다. 실제 값이 그보다 큰 것은 두 순서통계량이 양의 상관을 갖기 때문이다.

</details>`,
	},
}
