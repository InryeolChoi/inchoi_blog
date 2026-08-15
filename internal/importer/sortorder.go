package importer

import "sort"

// DenseRank는 값들을 오름차순으로 세우고 0부터 순위를 매긴다.
// 같은 값은 같은 순위를 받고, 다음 값은 바로 다음 순위로 이어진다.
//
//	["B", "A", "A", "C"]  →  [1, 0, 0, 2]
//
// 노션 created_time은 분 단위라 한 번에 만든 페이지들이 같은 값을 갖는다.
// 그런 페이지들은 순서를 알 수 없으므로 같은 순위로 둬서 "순서 미정"임을 남긴다.
// 억지로 다른 값을 주면 없는 순서를 지어내는 것이 된다.
func DenseRank(values []string) []int {
	uniq := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			uniq = append(uniq, v)
		}
	}
	sort.Strings(uniq)

	rank := make(map[string]int, len(uniq))
	for i, v := range uniq {
		rank[v] = i
	}

	out := make([]int, len(values))
	for i, v := range values {
		out[i] = rank[v]
	}
	return out
}

// TiedGroups는 같은 값을 가진 항목이 몇 쌍이나 되는지, 즉 순서를 정하지 못한
// 쌍의 수를 센다. 전체 쌍 수와 함께 돌려준다.
func TiedGroups(values []string) (tiedPairs, totalPairs int) {
	n := len(values)
	totalPairs = n * (n - 1) / 2

	counts := map[string]int{}
	for _, v := range values {
		counts[v]++
	}
	for _, c := range counts {
		tiedPairs += c * (c - 1) / 2
	}
	return tiedPairs, totalPairs
}

// AllTied는 값이 둘 이상인데 전부 같은지 본다. 이런 묶음은 순서를 전혀 알 수 없다.
func AllTied(values []string) bool {
	if len(values) < 2 {
		return false
	}
	for _, v := range values[1:] {
		if v != values[0] {
			return false
		}
	}
	return true
}
