package yamlgen_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateDeterministic 验证「同一份 yaml 多次生成，产物逐字节一致」。
//
// 回归背景：
//  1. Generate 早期直接 range yaml.TableMap，Go map 遍历顺序随机；
//  2. 关联目标表会先以「裸模型」（不带 relate/column 定制）占位，
//     而 gen.FieldRelate 会把目标表当时的 Relations() 快照进字段里。
//
// 两者叠加后，双向/环状关联的嵌套关系字段会时有时无，同一份 yaml
// 每次生成的代码都不一样。用例里 deliberately 构造 author ⇄ article 的
// 双向关联，并重复生成多次做比对。
func TestGenerateDeterministic(t *testing.T) {
	db := newTestDB(t,
		`CREATE TABLE author (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE article (id INTEGER PRIMARY KEY, author_id INTEGER, title TEXT)`,
		`CREATE TABLE comment (id INTEGER PRIMARY KEY, article_id INTEGER, content TEXT)`,
	)

	yamlContent := `
relation:
  - table: author
    relate:
      - table: article
        foreign_key: author_id
        references: id
        type: has_many
  - table: article
    relate:
      - table: author
        foreign_key: id
        references: author_id
        type: has_one
      - table: comment
        foreign_key: article_id
        references: id
        type: has_many
  - table: comment
    relate:
      - table: article
        foreign_key: id
        references: article_id
        type: has_one
`

	// 多跑几次：单次比对有可能因为万一撞上同样的遍历顺序而漏检
	const rounds = 5
	base := snapshotDir(t, generate(t, db, yamlContent))

	for i := 0; i < rounds; i++ {
		current := snapshotDir(t, generate(t, db, yamlContent))

		if len(current) != len(base) {
			t.Fatalf("第 %d 次生成的文件数量不一致: 期望 %d, 实际 %d", i+1, len(base), len(current))
		}
		for name, want := range base {
			got, ok := current[name]
			if !ok {
				t.Errorf("第 %d 次生成缺少文件: %s", i+1, name)
				continue
			}
			if got != want {
				t.Errorf("第 %d 次生成的文件与首次不一致: %s", i+1, name)
			}
		}
	}
}

// snapshotDir 递归读取目录下所有文件，返回「相对路径 -> 内容」的快照。
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()

	files := make(map[string]string)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("读取生成目录失败: %v", err)
	}
	return files
}
