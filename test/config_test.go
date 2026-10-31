package yamlgen_test

import (
	"regexp"
	"strings"
	"testing"
)

// TestTagJsonCamel 验证 config.tag_json_camel 对 json tag 的影响。
func TestTagJsonCamel(t *testing.T) {
	cases := []struct {
		name     string
		value    string // "" 表示不填
		wantJson string // 期望 user_name 字段的 json tag
	}{
		{"不填", "", `json:"user_name"`},
		{"lower", "lower", `json:"userName"`},
		{"upper", "upper", `json:"UserName"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t, `CREATE TABLE demo_user (id INTEGER PRIMARY KEY, user_name TEXT)`)

			config := ""
			if tc.value != "" {
				config = "config:\n  tag_json_camel: " + tc.value + "\n"
			}
			yamlContent := config + "relation:\n  - table: demo_user\n"

			outDir := generate(t, db, yamlContent)
			content := readEntity(t, outDir, "demo_user")

			if !strings.Contains(content, tc.wantJson) {
				t.Errorf("期望包含 %q，实际生成：\n%s", tc.wantJson, content)
			}
		})
	}
}

// TestColumnType 验证 column.type 对字段 Go 类型的影响。
func TestColumnType(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE demo_user (id INTEGER PRIMARY KEY, age INTEGER, name TEXT)`)

	yamlContent := `
relation:
  - table: demo_user
    column:
      age:
        type: int64
      name:
        type: string
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "demo_user")

	// age 被覆盖为 int64（用正则忽略 gofmt 的空格对齐）
	if !regexp.MustCompile(`Age\s+int64`).MatchString(content) {
		t.Errorf("期望 age 字段类型为 int64，实际：\n%s", content)
	}
}

// TestColumnRename 验证 column.rename 重命名字段。
func TestColumnRename(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE demo_user (id INTEGER PRIMARY KEY, deleted_at TEXT)`)

	yamlContent := `
relation:
  - table: demo_user
    column:
      deleted_at:
        rename: DeletedAt
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "demo_user")

	if !strings.Contains(content, "DeletedAt string") {
		t.Errorf("期望字段被重命名为 DeletedAt，实际：\n%s", content)
	}
}

// TestColumnComment 验证 column.comment 生成字段注释。
func TestColumnComment(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE demo_user (id INTEGER PRIMARY KEY, name TEXT)`)

	yamlContent := `
relation:
  - table: demo_user
    column:
      name:
        comment: 用户名
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "demo_user")

	if !strings.Contains(content, "用户名") {
		t.Errorf("期望包含注释「用户名」，实际：\n%s", content)
	}
}

// TestColumnTag 验证 column.tag 自定义 struct tag。
func TestColumnTag(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE demo_user (id INTEGER PRIMARY KEY, status TEXT)`)

	yamlContent := `
relation:
  - table: demo_user
    column:
      status:
        tag:
          gorm:
            default: "'active'"
          json:
            omitempty: ""
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "demo_user")

	if !strings.Contains(content, `default:'active'`) {
		t.Errorf("期望包含 gorm default tag，实际：\n%s", content)
	}
	// 空 value 的 tag（omitempty: ""）会生成无值形式 json:"omitempty"
	if !strings.Contains(content, `json:"omitempty"`) {
		t.Errorf("期望包含 json omitempty tag，实际：\n%s", content)
	}
}

