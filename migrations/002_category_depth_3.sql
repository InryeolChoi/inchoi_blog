-- 002_category_depth_3.sql — 카테고리를 3단계까지 허용하고, 깊이 검사를 제대로 고친다.
--
-- 왜 3단계인가:
--   노션 최상위 19개를 더 큰 분류 8개(algorithm, cs-theory, dev, …) 밑으로 넣으면서
--   8 > 19 > 66 구조가 됐다. 66개 하위 분류와 거기 붙은 글 1273건을 살리려면
--   3단계가 필요하다. 2단계를 고집하면 66개를 버리거나 19개를 버려야 한다.
--
-- 001의 트리거에 있던 구멍:
--   "새 부모가 이미 자식인가"만 봤다. 옮기는 노드가 자식을 데리고 있는지는 보지 않아서,
--   자식이 있는 최상위 노드를 다른 최상위 밑으로 옮기면 3단계가 조용히 생겼다.
--   막으라고 만든 것이 안 막고 있었다.
--
-- 새 검사는 양방향을 다 본다:
--   새 깊이 = (새 부모의 깊이) + 1 + (옮기는 노드가 데리고 가는 서브트리 높이)
--   이 값이 3을 넘으면 막는다.
--
-- 최대 깊이가 3이라 재귀 없이 두 단계만 내려다보면 된다.
-- 깊이/높이가 3을 넘는 상태는 이 트리거들이 애초에 만들지 못하게 하므로,
-- CASE의 마지막 ELSE가 "3 이상"을 뭉뚱그려도 판정이 틀리지 않는다.

DROP TRIGGER categories_max_depth_on_insert;
DROP TRIGGER categories_max_depth_on_update;

-- 새로 넣는 행은 자식이 없으므로 부모의 깊이만 보면 된다.
-- 부모가 이미 깊이 3이면 그 밑은 4단계가 된다.
CREATE TRIGGER categories_max_depth_on_insert
BEFORE INSERT ON categories
WHEN NEW.parent_id IS NOT NULL
     AND (SELECT p.parent_id FROM categories p WHERE p.id = NEW.parent_id) IS NOT NULL
     AND (SELECT pp.parent_id
            FROM categories p
            JOIN categories pp ON p.parent_id = pp.id
           WHERE p.id = NEW.parent_id) IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'categories: 최대 3단계까지만 허용');
END;

-- 옮길 때는 딸려가는 서브트리까지 계산한다.
CREATE TRIGGER categories_max_depth_on_update
BEFORE UPDATE OF parent_id ON categories
WHEN NEW.parent_id IS NOT NULL
     AND (
       -- 새 부모의 깊이 (1 = 최상위)
       (CASE
          WHEN (SELECT p.parent_id FROM categories p WHERE p.id = NEW.parent_id) IS NULL THEN 1
          WHEN (SELECT pp.parent_id
                  FROM categories p
                  JOIN categories pp ON p.parent_id = pp.id
                 WHERE p.id = NEW.parent_id) IS NULL THEN 2
          ELSE 3
        END)
       + 1
       -- 옮기는 노드의 서브트리 높이 (0 = 자식 없음)
       + (CASE
            WHEN NOT EXISTS (SELECT 1 FROM categories c WHERE c.parent_id = NEW.id) THEN 0
            WHEN NOT EXISTS (SELECT 1
                               FROM categories g
                               JOIN categories c ON g.parent_id = c.id
                              WHERE c.parent_id = NEW.id) THEN 1
            ELSE 2
          END)
       > 3
     )
BEGIN
    SELECT RAISE(ABORT, 'categories: 최대 3단계까지만 허용 (옮기는 노드의 자식까지 계산함)');
END;

-- 자기 자신을 부모로 삼으면 깊이 계산이 무한히 돈다. 위 트리거로는 안 걸린다
-- (최상위 노드의 parent_id를 자기 id로 바꾸면 "새 부모의 깊이"가 1로 나온다).
CREATE TRIGGER categories_no_self_parent
BEFORE UPDATE OF parent_id ON categories
WHEN NEW.parent_id = NEW.id
BEGIN
    SELECT RAISE(ABORT, 'categories: 자기 자신을 부모로 삼을 수 없다');
END;
