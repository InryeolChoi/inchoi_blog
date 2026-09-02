package curation

// `알고리즘 : 그래프 (심화)` 글 손질(2026-09-02).
//
// 이 분류에는 제목만 있고 내용이 없는 자리가 둘 있었다.
//
//	가중치 그래프    본문이 "가중치가 있는 그래프를 다뤄보자" 한 줄(20자)
//	그래프 탐색 유형  절 제목 다섯 개만 있고 그 아래가 전부 비어 있었다
//
// **`그래프 탐색 유형`은 draft가 아니라 unlisted였다.** 제목 다섯 줄만 있는
// 페이지가 공개 화면에 그대로 나가고 있었다는 뜻이다.
var graphAlgorithmEdits = buildGraphAlgorithmEdits()

func buildGraphAlgorithmEdits() []BodyEdit {
	type step struct{ pageID, title, remove, replace, why string }

	const (
		weighted = "57fc8387-caff-4b32-a6c3-7ec5723e9e21" // 가중치 그래프
		patterns = "3bc18a6c-1108-4996-abc7-8bee7262a80f" // 그래프 탐색 유형
	)

	steps := []step{
		{weighted, "가중치 그래프",
			"- 가중치가 있는 그래프를 다뤄보자", weightedGraphBody,
			"본문이 한 줄뿐이었다. 가중치 그래프가 무엇이고 어떤 알고리즘으로 이어지는지 채운다"},

		{patterns, "그래프 탐색 유형",
			"## 1. 방문(전파) 가능한 노드 갯수 구하기", patternReach,
			"제목만 있고 내용이 없었다"},
		{patterns, "그래프 탐색 유형",
			"## 2. 목적지까지 최단거리", patternShortest,
			"제목만 있고 내용이 없었다"},
		{patterns, "그래프 탐색 유형",
			"## 3. 독립된 지역의 갯수 구하기", patternComponents,
			"제목만 있고 내용이 없었다"},
		{patterns, "그래프 탐색 유형",
			"## 4. 정상 노드 감염시키기", patternInfect,
			"제목만 있고 내용이 없었다"},
		{patterns, "그래프 탐색 유형",
			"## 5. 동일 위치에 재 방문이 가능한 경우", patternRevisit,
			"제목만 있고 내용이 없었다"},
	}

	out := make([]BodyEdit, 0, len(steps))
	for _, s := range steps {
		out = append(out, BodyEdit{
			NotionPageID: s.pageID, Remove: s.remove, Replace: s.replace,
			Title: s.title, Why: s.why,
		})
	}
	return out
}

const weightedGraphBody = `# 가중치 그래프란?

간선마다 **비용**(weight)이 붙어 있는 그래프다. 지금까지의 그래프는 "이어져 있다/아니다"만 따졌지만, 현실의 문제는 대개 "얼마나 멀리" 또는 "얼마나 비싸게" 이어져 있느냐를 묻는다.

- 도로망 : 도시와 도시 사이의 거리
- 네트워크 : 회선의 지연 시간
- 작업 : 공정 사이에 걸리는 시간

간선 $(u, v)$의 가중치를 $w(u, v)$로 적는다.

## 표현 방법

**인접 행렬.** $A[u][v]$에 가중치를 넣는다. 간선이 없으면 $\infty$(코드에서는 아주 큰 수)로 둔다. 정점이 $V$개면 $O(V^2)$의 공간을 쓰므로 **간선이 빽빽할 때** 유리하다.

$$
A[u][v] =
\begin{cases}
w(u,v) & (u,v)\text{가 간선일 때} \\
\infty & \text{아닐 때}
\end{cases}
$$

**인접 리스트.** 정점마다 ` + "`(이웃, 가중치)`" + ` 쌍을 담는다. 공간이 $O(V+E)$라 **간선이 성길 때** 유리하고, 실제 문제 풀이에서는 대개 이쪽을 쓴다.

` + "```python" + `
# 무방향 그래프. 방향 그래프면 한쪽만 넣는다.
graph = [[] for _ in range(n + 1)]
for u, v, w in edges:
    graph[u].append((v, w))
    graph[v].append((u, w))
` + "```" + `

## 왜 BFS로는 안 되는가

간선의 가중치가 **모두 같을 때만** BFS가 최단거리를 준다. BFS는 "간선을 몇 개 지났나"로 거리를 세기 때문이다. 가중치가 다르면 간선 수가 적은 길이 오히려 더 비쌀 수 있다.

$$
1 \xrightarrow{\ 10\ } 3, \qquad 1 \xrightarrow{\ 1\ } 2 \xrightarrow{\ 1\ } 3
$$

BFS는 간선 하나짜리 위쪽 길(비용 10)을 먼저 찾지만, 실제 최단거리는 아래쪽 길(비용 2)이다. 그래서 **가중치 그래프에는 따로 알고리즘이 필요하다.**

## 어떤 알고리즘으로 이어지나

| 문제 | 알고리즘 | 시간복잡도 | 음수 간선 |
|---|---|---|---|
| 한 점에서 모든 점까지 | 다익스트라 | $O(E\log V)$ | 안 됨 |
| 한 점에서 모든 점까지 | 벨만-포드 | $O(VE)$ | 됨 (음수 순환 탐지) |
| 모든 점 쌍 사이 | 플로이드-워셜 | $O(V^3)$ | 됨 |
| 모든 정점을 잇는 최소 비용 | 크루스칼 · 프림 | $O(E\log E)$ | 됨 |

**음수 간선이 있으면 다익스트라를 쓸 수 없다.** 다익스트라는 "지금까지 확정된 거리는 더 줄지 않는다"를 전제로 정점을 하나씩 확정하는데, 음수 간선이 있으면 나중에 더 짧은 길이 나타날 수 있어 그 전제가 깨진다.

바로 다음 글에서 **다익스트라 알고리즘**을 다룬다.`

const patternReach = `## 1. 방문(전파) 가능한 노드 갯수 구하기

한 정점에서 출발해 **닿을 수 있는 정점이 몇 개인가**를 묻는 유형이다.

탐색을 한 번 돌리면서 방문 표시를 세면 끝이라, **DFS든 BFS든 상관없다.** 거리를 묻지 않기 때문이다.

` + "```python" + `
def count_reachable(graph, start):
    visited = [False] * len(graph)
    stack = [start]
    visited[start] = True
    count = 0
    while stack:
        node = stack.pop()
        count += 1
        for nxt in graph[node]:
            if not visited[nxt]:
                visited[nxt] = True      # 넣을 때 표시한다
                stack.append(nxt)
    return count
` + "```" + `

**방문 표시는 큐에 넣을 때 한다.** 꺼낼 때 표시하면 같은 정점이 큐에 여러 번 들어가 시간이 늘고, 최악에는 간선 수만큼 중복된다.

출발점을 셀지 말지는 문제가 정한다. "출발점을 제외한 감염 노드 수" 같은 조건이면 $-1$을 해준다.`

const patternShortest = `## 2. 목적지까지 최단거리

시작점에서 도착점까지 **간선을 최소 몇 개 지나는가**를 묻는 유형이다.

**여기서는 반드시 BFS다.** BFS는 가까운 곳부터 층층이 넓혀 가므로, 어떤 정점에 처음 닿은 순간이 곧 그 정점까지의 최단거리다. DFS는 깊이 먼저 파고들어서 먼저 닿은 길이 최단이라는 보장이 없다.

` + "```python" + `
from collections import deque

def shortest(graph, start, goal):
    dist = [-1] * len(graph)
    dist[start] = 0
    q = deque([start])
    while q:
        node = q.popleft()
        if node == goal:
            return dist[node]
        for nxt in graph[node]:
            if dist[nxt] == -1:
                dist[nxt] = dist[node] + 1
                q.append(nxt)
    return -1        # 닿을 수 없다
` + "```" + `

방문 배열을 따로 두지 않고 ` + "`dist`" + `의 $-1$ 여부로 겸한다. 두 배열을 따로 두면 한쪽만 갱신하는 실수가 난다.

**간선마다 비용이 다르면 이 방법이 통하지 않는다.** 그때는 다익스트라를 쓴다 — ` + "`가중치 그래프`" + ` 글 참고.`

const patternComponents = `## 3. 독립된 지역의 갯수 구하기

서로 이어지지 않은 덩어리가 **몇 개인가**를 묻는 유형이다. 연결 요소(connected component)를 세는 문제다.

모든 정점을 훑으면서 **아직 방문하지 않은 정점을 만날 때마다** 탐색을 새로 시작하고, 그 횟수를 센다. 한 번의 탐색이 한 덩어리를 통째로 지우기 때문이다.

` + "```python" + `
def count_components(graph):
    visited = [False] * len(graph)
    count = 0
    for v in range(len(graph)):
        if visited[v]:
            continue
        count += 1              # 새 덩어리를 만났다
        stack = [v]
        visited[v] = True
        while stack:
            node = stack.pop()
            for nxt in graph[node]:
                if not visited[nxt]:
                    visited[nxt] = True
                    stack.append(nxt)
    return count
` + "```" + `

격자(2차원 배열)로 주어지는 문제도 같은 꼴이다. 섬의 개수, 단지 수, 그림 개수 같은 것들인데 **이웃을 상하좌우 네 방향(또는 대각선까지 여덟 방향)으로 정의**하는 것만 다르다.

바깥 반복문이 정점을 한 번씩 훑고 안쪽 탐색이 간선을 한 번씩 보므로 전체 $O(V+E)$다.`

const patternInfect = `## 4. 정상 노드 감염시키기

한 정점에서 시작해 **몇 번 만에 전체로 퍼지는가**, 또는 **시간 $t$에 몇 개가 감염되는가**를 묻는 유형이다.

닿는 것만 세는 1번과 달리 **몇 단계인지**를 함께 따지므로 BFS를 쓴다. 층(level)을 세는 것이 곧 시간이다.

` + "```python" + `
from collections import deque

def infect(graph, start):
    dist = [-1] * len(graph)
    dist[start] = 0
    q = deque([start])
    last = 0
    while q:
        node = q.popleft()
        last = dist[node]
        for nxt in graph[node]:
            if dist[nxt] == -1:
                dist[nxt] = dist[node] + 1
                q.append(nxt)
    return last          # 전부 퍼지는 데 걸린 시간
` + "```" + `

**출발점이 여럿이면 큐에 한꺼번에 넣고 시작한다.** 하나씩 따로 BFS를 돌려 최솟값을 취하는 것보다 훨씬 빠르다 — 여러 곳에서 동시에 번지는 상황을 한 번의 탐색으로 처리한다(multi-source BFS).

` + "```python" + `
q = deque(sources)
for s in sources:
    dist[s] = 0
` + "```" + `

퍼지지 못한 정점이 남았는지는 마지막에 ` + "`dist`" + `에 $-1$이 있는지로 본다.`

const patternRevisit = `## 5. 동일 위치에 재 방문이 가능한 경우

같은 정점을 **다시 밟아도 되는** 문제다. 이때 "정점 하나에 방문 표시 하나"로는 풀리지 않는다.

핵심은 **무엇을 상태로 볼 것인가**다. 위치만으로는 부족하고, 문제가 주는 조건을 함께 묶어야 한다.

- 벽을 $k$번까지 부술 수 있다 → 상태 = ` + "`(위치, 부순 횟수)`" + `
- 낮과 밤에 규칙이 다르다 → 상태 = ` + "`(위치, 시간 % 2)`" + `
- 열쇠를 모아야 문이 열린다 → 상태 = ` + "`(위치, 가진 열쇠 비트마스크)`" + `

방문 배열도 그 상태만큼 차원을 늘린다.

` + "```python" + `
from collections import deque

# 벽을 최대 k번 부수며 (0,0) → (n-1,m-1) 최단거리
def bfs(grid, n, m, k):
    visited = [[[False] * (k + 1) for _ in range(m)] for _ in range(n)]
    q = deque([(0, 0, 0, 1)])        # y, x, 부순 횟수, 거리
    visited[0][0][0] = True
    while q:
        y, x, broken, d = q.popleft()
        if (y, x) == (n - 1, m - 1):
            return d
        for ny, nx in neighbors(y, x, n, m):
            if grid[ny][nx] == 0 and not visited[ny][nx][broken]:
                visited[ny][nx][broken] = True
                q.append((ny, nx, broken, d + 1))
            elif grid[ny][nx] == 1 and broken < k and not visited[ny][nx][broken + 1]:
                visited[ny][nx][broken + 1] = True
                q.append((ny, nx, broken + 1, d + 1))
    return -1
` + "```" + `

**같은 칸이라도 상태가 다르면 다른 자리다.** 벽을 0번 부수고 도착한 칸과 2번 부수고 도착한 칸은 앞으로 갈 수 있는 길이 다르므로 따로 방문 표시를 둬야 한다. 이걸 합치면 더 나은 경로를 잃는다.

상태 수가 늘면 시간도 그만큼 는다. 위 예는 $O(n \times m \times k)$다.`
