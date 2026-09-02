package curation

// mathstat2ExamAppends는 수리통계2 `수리통계2 - 시험` 글의 중간고사·기말고사
// 전 문항 해답을 본문 끝에 덧붙인다. BodyAppend 타입은
// probability_process_solutions.go에 정의돼 있다.
var mathstat2ExamAppends = []BodyAppend{
	{
		NotionPageID: "1f3d0731-e367-4d0d-8239-94d92d6d02d5",
		Marker:       "## 중간고사 해답",
		Title:        "수리통계2 - 시험",
		Why:          "수리통계2 중간·기말고사 전 문항 해답을 작성했다",
		Markdown: `## 중간고사 해답

<details>
<summary><strong>중간고사 해답 보기</strong></summary>

### 1. 균등분포 $\theta$의 적률법 추정량과 최대우도추정량

$X_i\overset{\mathrm{iid}}\sim\text{Uniform}(1,\theta)$, $\theta>1$, $i=1,\ldots,n$이고 밀도는

$$
f(x;\theta)=\frac{1}{\theta-1},\qquad 1<x<\theta
$$

이다.

**적률법(MoM).** $E[X]=\dfrac{1+\theta}{2}$이므로 표본적률과 모적률을 같다고 놓으면

$$
\frac{1+\theta}{2}=\bar X
\quad\Longrightarrow\quad
\boxed{\tilde\theta_{MoM}=2\bar X-1}
$$

이다.

**최대우도추정량(MLE).** 가능도함수는

$$
L(\theta)=\prod_{i=1}^n\frac{1}{\theta-1}\,I(1<x_i<\theta)
=(\theta-1)^{-n}\,I\!\left(\theta>\max_i x_i\right)
$$

이다. $\theta>\max_i x_i$인 범위에서 $L(\theta)$는 $\theta$에 대해 감소함수이므로 가능도를 최대화하는 $\theta$는 그 범위의 가장 작은 값이다. 즉

$$
\boxed{\hat\theta_{MLE}=X_{(n)}=\max_{1\le i\le n}X_i}
$$

이다.

**두 추정량의 비교.** $\tilde\theta_{MoM}=2\bar X-1$은

$$
E[\tilde\theta_{MoM}]=2\cdot\frac{1+\theta}{2}-1=\theta
$$

이므로 불편추정량이고,

$$
\operatorname{Var}(\tilde\theta_{MoM})=4\operatorname{Var}(\bar X)=\frac4n\operatorname{Var}(X)=\frac4n\cdot\frac{(\theta-1)^2}{12}=\frac{(\theta-1)^2}{3n}
$$

이므로

$$
\text{MSE}(\tilde\theta_{MoM})=\frac{(\theta-1)^2}{3n}=O(1/n)
$$

이다. 한편 $X_{(n)}$의 분포함수는 $1<t<\theta$에서 $F_{(n)}(t)=\left(\dfrac{t-1}{\theta-1}\right)^{n}$이고, $u=t-1$로 치환해 직접 적분하면(균등분포 최댓값의 표준 결과)

$$
E[X_{(n)}]=1+\frac{n}{n+1}(\theta-1)=\theta-\frac{\theta-1}{n+1},
\qquad
\operatorname{Var}(X_{(n)})=\frac{n(\theta-1)^2}{(n+1)^2(n+2)}
$$

를 얻는다. 따라서 $\hat\theta_{MLE}$는 편향

$$
\operatorname{Bias}(\hat\theta_{MLE})=-\frac{\theta-1}{n+1}
$$

을 갖는 편의추정량이지만 $n\to\infty$일 때 편향이 0으로 사라지고,

$$
\text{MSE}(\hat\theta_{MLE})
=\operatorname{Var}(X_{(n)})+\operatorname{Bias}(\hat\theta_{MLE})^2
=\frac{n(\theta-1)^2}{(n+1)^2(n+2)}+\frac{(\theta-1)^2}{(n+1)^2}
=\frac{2(\theta-1)^2}{(n+1)(n+2)}=O(1/n^2)
$$

이다. 두 MSE의 비를 보면

$$
\boxed{
\frac{\text{MSE}(\hat\theta_{MLE})}{\text{MSE}(\tilde\theta_{MoM})}
=\frac{6n}{(n+1)(n+2)}\xrightarrow[n\to\infty]{}0
}
$$

이 되어, MLE는 편의추정량임에도 MoM 추정량보다 훨씬 빠른 $O(1/n^2)$ 수렴 속도를 가져 평균제곱오차 기준으로 훨씬 우수하다. 지지집합의 끝점이 모수 $\theta$에 의존하는 비정칙(non-regular) 모형이라 MLE가 통상적인 $O(1/n)$의 효율 한계를 넘어서는 경우다.

### 2. 정규분포 분산의 최대우도추정량의 분산

$X_i\overset{\mathrm{iid}}\sim N(\mu,\sigma^2)$, $\mu$는 알려져 있다, $i=1,\ldots,n$. 최대우도추정량은

$$
\hat\sigma^2_{MLE}=\frac1n\sum_{i=1}^n(X_i-\mu)^2
$$

이다. $Z_i=(X_i-\mu)/\sigma\overset{\mathrm{iid}}\sim N(0,1)$이라 하면

$$
\hat\sigma^2_{MLE}=\frac{\sigma^2}{n}\sum_{i=1}^n Z_i^2,
\qquad
\sum_{i=1}^n Z_i^2\sim\chi^2_n
$$

이고 $\operatorname{Var}(\chi^2_n)=2n$이므로

$$
\operatorname{Var}(\hat\sigma^2_{MLE})=\frac{\sigma^4}{n^2}\cdot\operatorname{Var}(\chi^2_n)=\frac{\sigma^4}{n^2}\cdot2n
$$

$$
\boxed{\operatorname{Var}(\hat\sigma^2_{MLE})=\frac{2\sigma^4}{n}}
$$

이다.

### 3. Bayes estimator

$X_i\mid\mu\overset{\mathrm{iid}}\sim N(\mu,100)$, $i=1,\ldots,n=10$이고, 사전분포는 $\mu\sim\text{Uniform}(10,15)$, 즉 $\pi(\mu)=\dfrac15I(10<\mu<15)$이다.

가능도는

$$
L(\mu)\propto\exp\left(-\frac{1}{200}\sum_{i=1}^{10}(x_i-\mu)^2\right)
\propto\exp\left(-\frac{10(\mu-\bar x)^2}{200}\right)
=\exp\left(-\frac{(\mu-\bar x)^2}{20}\right)
$$

이다($\sum(x_i-\mu)^2=\sum(x_i-\bar x)^2+n(\bar x-\mu)^2$에서 $\mu$와 무관한 항을 뺐다). 이는 $\mu$에 대해 평균 $\bar x$, 분산 $\sigma^2/n=100/10=10$인 정규분포의 커널과 같다. 사전분포가 $(10,15)$에서 균등이므로 사후분포는

$$
\pi(\mu\mid\mathbf x)\propto\exp\left(-\frac{(\mu-\bar x)^2}{20}\right)I(10<\mu<15)
$$

즉 평균 $\bar x$, 분산 10인 정규분포를 $(10,15)$ 구간으로 절단한 절단정규분포(truncated normal)다.

제곱오차손실 하에서 베이즈 추정량은 사후평균이다. 절단정규분포의 평균 공식(평균 $\mu_0$, 표준편차 $\sigma_0$인 정규분포를 $(a,b)$로 절단하면 $E[Y]=\mu_0+\sigma_0\dfrac{\phi(\alpha)-\phi(\beta)}{\Phi(\beta)-\Phi(\alpha)}$, $\alpha=(a-\mu_0)/\sigma_0$, $\beta=(b-\mu_0)/\sigma_0$)에 $\mu_0=\bar x$, $\sigma_0=\sqrt{10}$, $a=10$, $b=15$를 대입하면

$$
\boxed{
\hat\mu_{Bayes}=E[\mu\mid\mathbf x]
=\bar x+\sqrt{10}\,
\frac{
\phi\!\left(\dfrac{10-\bar x}{\sqrt{10}}\right)
-\phi\!\left(\dfrac{15-\bar x}{\sqrt{10}}\right)
}{
\Phi\!\left(\dfrac{15-\bar x}{\sqrt{10}}\right)
-\Phi\!\left(\dfrac{10-\bar x}{\sqrt{10}}\right)
}
}
$$

이다. $\phi,\Phi$는 각각 표준정규분포의 밀도함수와 분포함수다. 표본평균 $\bar x$가 $(10,15)$ 안쪽 깊숙이 있고 사후표준편차 $\sqrt{10}\approx3.16$에 비해 경계까지 충분히 멀면 절단의 효과가 작아져 $\hat\mu_{Bayes}\approx\bar x$에 가까워진다.

### 4. 모평균의 95% 신뢰구간

자료는 $45,\ 20,\ 10,\ 12,\ 25$이고 $n=5$다.

$$
\bar x=\frac{45+20+10+12+25}{5}=\frac{112}{5}=22.4
$$

$$
\sum_{i=1}^5(x_i-\bar x)^2
=(22.6)^2+(-2.4)^2+(-12.4)^2+(-10.4)^2+(2.6)^2
=510.76+5.76+153.76+108.16+6.76=785.2
$$

$$
s^2=\frac{785.2}{n-1}=\frac{785.2}{4}=196.3,
\qquad
s=\sqrt{196.3}\approx14.011
$$

모분산을 모르는 정규모집단에서 모평균의 $100(1-\alpha)\%$ 신뢰구간은 자유도 $n-1=4$인 $t$분포를 써서

$$
\bar X\pm t_{\alpha/2,\,n-1}\,\frac{S}{\sqrt n}
$$

이다. 여기서 힌트로 주어진 값은 $t_{0.025}(5)=2.571$인데, 이 값은 자유도가 5일 때의 값이다($n=5$에서 정석대로라면 자유도는 $n-1=4$이고 $t_{0.025}(4)=2.776$이다). 아래에서는 **문제가 제시한 힌트값을 그대로** 써서 계산한다.

$$
\frac{s}{\sqrt n}=\frac{14.011}{\sqrt5}\approx6.266,
\qquad
t_{0.025}\cdot\frac{s}{\sqrt n}\approx2.571\times6.266\approx16.11
$$

이므로 95% 신뢰구간은

$$
22.4\pm16.11
$$

$$
\boxed{(6.29,\ 38.51)}
$$

이다. 이 구간의 해석은 다음과 같다: 같은 방식으로 표본을 반복해서 뽑아 신뢰구간을 만드는 절차를 무한히 반복하면, 그렇게 만들어진 구간들 중 약 95%가 모평균 $\mu$의 참값을 포함한다는 뜻이다. 하나의 표본에서 얻은 이 특정 구간이 95%의 확률로 $\mu$를 포함한다는 뜻이 아니다.

</details>`,
	},
	{
		NotionPageID: "1f3d0731-e367-4d0d-8239-94d92d6d02d5",
		Marker:       "## 기말고사 해답",
		Title:        "수리통계2 - 시험",
		Why:          "수리통계2 중간·기말고사 전 문항 해답을 작성했다",
		Markdown: `## 기말고사 해답

<details>
<summary><strong>기말고사 해답 보기</strong></summary>

### 1. 혈액형 자료의 최강력검정(Most Powerful Test)

확률변수 $X_i$는 $i$번째 환자의 혈액형(1: A형, 2: B형, 3: AB형, 4: O형)이고

$$
P(X_i=x)=p_1I(x=1)+p_2I(x=2)+p_3I(x=3)+(1-p_1-p_2-p_3)I(x=4)
$$

이다. $X_1,\ldots,X_n$은 서로 독립이고, 검정하려는 가설은

$$
H_0:p_1=\frac13\qquad\text{vs}\qquad H_1:p_1=\frac12
$$

이다. $p_2,p_3$는 두 가설 모두에서 값이 바뀌지 않는 성가신 모수로 둔다.

$Y_i=I(X_i=1)$(환자가 A형인지 여부)라 하면, $X_i\neq1$일 때 그 조건부 분포(B형·AB형·O형 사이의 비율)는 $p_1$과 무관한 상수이므로 전체 가능도는

$$
L(p_1)=\prod_{i=1}^np_1^{I(x_i=1)}(1-p_1)^{1-I(x_i=1)}\times C
$$

로 인수분해된다. 여기서 $C=\prod_{i:x_i\neq1}q_{x_i}$($q_2,q_3,q_4$는 A형이 아닐 때의 조건부 확률)는 $p_1$과 무관한 상수다. $T=\sum_{i=1}^nY_i$(A형 환자의 수)라 하면

$$
L(p_1)=C\cdot p_1^{T}(1-p_1)^{n-T}
$$

이다. Neyman–Pearson 정리에 의해 최강력검정은 가능도비

$$
\Lambda(\mathbf x)=\frac{L(1/2)}{L(1/3)}
=\frac{(1/2)^{T}(1/2)^{n-T}}{(1/3)^{T}(2/3)^{n-T}}
=\left(\frac34\right)^{n}2^{T}
$$

가 클 때 $H_0$를 기각한다. $\Lambda(\mathbf x)$는 $T$에 대해 증가함수이므로 기각역은 $T\ge c$ 꼴이다:

$$
\text{기각역: }\quad T=\sum_{i=1}^nI(X_i=1)\ge c
$$

$c$는 $H_0$ 하에서 $T\sim\text{Binomial}(n,1/3)$이고 $P_{p_1=1/3}(T\ge c)=0.05$가 되도록 정한다. $n$이 커서 정규근사를 쓰면 $E_{H_0}[T]=n/3$, $\operatorname{Var}_{H_0}(T)=n\cdot\frac13\cdot\frac23=\frac{2n}{9}$이므로

$$
\boxed{
\text{기각역: }\quad
\frac{T-n/3}{\sqrt{2n/9}}\ge z_{0.05}=1.645
\quad\Longleftrightarrow\quad
T\ge\frac n3+1.645\sqrt{\frac{2n}{9}}
}
$$

이다. $\Lambda(\mathbf x)$가 $T$에 대해 단조증가하므로 이 검정은 $H_1$의 특정값 $p_1=1/2$에 의존하지 않고 $p_1>1/3$인 모든 값에 대해 최강력이다. 즉 단조가능도비(MLR) 성질에 의해 Karlin–Rubin 정리로 $H_0:p_1\le1/3$ vs $H_1:p_1>1/3$에 대한 균등최강력검정(UMP test)이기도 하다.

### 2. 과체중 비율에 대한 가설검정

$Y_i=I(\text{BMI}_i\ge25)$(과체중 여부), $i=1,\ldots,n$은 서로 독립이고 $\text{Bernoulli}(p)$를 따른다. 단, $p=P(\text{과체중})$이다. 과체중일 확률이 $1/3$을 초과하는지 확인하려는 것이므로

$$
H_0:p\le\frac13\qquad\text{vs}\qquad H_1:p>\frac13
$$

인 단측검정을 구성한다. 이항분포족 $\{\text{Bernoulli}(p)\}$는 위 1번 문제와 같은 방식으로 $T=\sum_{i=1}^nY_i$에 대해 단조가능도비(MLR)를 가지므로, Karlin–Rubin 정리에 의해 수준 $\alpha$의 균등최강력검정이 존재하고 그 형태는

$$
\text{기각역: }\quad T\ge c
$$

이다. $c$는 경계값 $p=1/3$에서 $P(T\ge c)=\alpha$가 되도록 정한다(경계에서 크기를 $\alpha$로 맞추면 $p<1/3$인 모든 점에서 검정의 크기가 $\alpha$ 이하가 된다).

$n$이 크면 정규근사를 써서 $\hat p=T/n$이라 할 때 검정통계량을

$$
Z=\frac{\hat p-1/3}{\sqrt{(1/3)(2/3)/n}}=\frac{\hat p-1/3}{\sqrt{2/(9n)}}
$$

로 두고,

$$
\boxed{
\text{유의수준 }\alpha\text{에서 기각역: }\quad Z\ge z_\alpha
}
$$

이다(예를 들어 $\alpha=0.05$이면 $z_{0.05}=1.645$). 표본에서 관측된 과체중 비율 $\hat p$가 $1/3$보다 충분히 크면 "과체중일 확률이 $1/3$을 초과한다"는 $H_1$을 채택한다.

### 3. 가능도비 검정(Likelihood Ratio Test)

$X_k$, $k=1,\ldots,n$은 서로 독립이고 $E[X_k]=k\theta$, $\operatorname{Var}(X_k)=25k^2\theta^2$($\theta>0$)이다. 분포의 구체적인 형태가 별도로 주어지지 않았으므로, 주어진 평균·분산만으로 가능도를 구성할 수 있는 가장 표준적인 가정인 정규분포를 따른다고 둔다:

$$
X_k\sim N(k\theta,\ 25k^2\theta^2),\qquad k=1,\ldots,n\ (\text{서로 독립})
$$

$W_k=X_k/k$라 하면

$$
E[W_k]=\theta,\qquad\operatorname{Var}(W_k)=25\theta^2
$$

로 $k$에 무관하다. 즉

$$
W_k=\frac{X_k}{k}\overset{\mathrm{iid}}\sim N(\theta,\ 25\theta^2),\qquad k=1,\ldots,n
$$

이다. $\overline W=\frac1n\sum_{k=1}^nW_k$, $\overline{W^2}=\frac1n\sum_{k=1}^nW_k^2$라 두면 로그가능도는

$$
\ell(\theta)=-n\ln\theta-\frac{1}{50\theta^2}\sum_{k=1}^n(W_k-\theta)^2+\text{const.}
=-n\ln\theta-\frac{n\overline{W^2}}{50\theta^2}+\frac{n\overline W}{25\theta}+\text{const.}
$$

이다(상수항은 $\theta$와 무관하다).

**MLE.** $\ell'(\theta)=0$을 풀면

$$
-\frac n\theta+\frac{n\overline{W^2}}{25\theta^3}-\frac{n\overline W}{25\theta^2}=0
$$

이고, 양변에 $\theta^3$을 곱해 정리하면

$$
25\theta^2+\overline W\,\theta-\overline{W^2}=0
$$

이다. 이 이차방정식의 두 근의 곱은 $-\overline{W^2}/25\le0$이므로 부호가 다른 두 실근을 갖는다. $\theta>0$을 가정했으므로 양의 근을 취하면

$$
\boxed{
\hat\theta=\frac{-\overline W+\sqrt{\overline W^{2}+100\,\overline{W^2}}}{50}
}
$$

를 얻는다.

**가능도비 검정통계량.** 가능도비는

$$
\Lambda(\mathbf x)=\frac{L(\theta_0)}{L(\hat\theta)}=\exp\bigl[\ell(\theta_0)-\ell(\hat\theta)\bigr]
$$

이고, 모수가 1차원이므로 Wilks 정리에 의해 $H_0$ 하에서

$$
-2\ln\Lambda(\mathbf X)\ \xrightarrow{d}\ \chi^2_{1}
$$

로 점근분포한다. $\ell(\theta)$의 식을 대입해 정리하면

$$
-2\ln\Lambda(\mathbf x)
=2n\left[
\ln\frac{\theta_0}{\hat\theta}
+\frac{\overline{W^2}}{50}\left(\frac1{\theta_0^2}-\frac1{\hat\theta^2}\right)
+\frac{\overline W}{25}\left(\frac1{\hat\theta}-\frac1{\theta_0}\right)
\right]
$$

이다. 따라서 유의수준 $\alpha$에서 가능도비 검정의 기각역은

$$
\boxed{
2n\left[
\ln\frac{\theta_0}{\hat\theta}
+\frac{\overline{W^2}}{50}\left(\frac1{\theta_0^2}-\frac1{\hat\theta^2}\right)
+\frac{\overline W}{25}\left(\frac1{\hat\theta}-\frac1{\theta_0}\right)
\right]
>\chi^2_{1,\alpha}
}
$$

이다. 여기서 $W_k=X_k/k$, $\overline W=\frac1n\sum_kW_k$, $\overline{W^2}=\frac1n\sum_kW_k^2$, $\hat\theta$는 위에서 구한 MLE이고 $\chi^2_{1,\alpha}$는 자유도 1인 카이제곱분포의 $100(1-\alpha)$ 백분위수다.

</details>`,
	},
}
