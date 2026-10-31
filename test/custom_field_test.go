package yamlgen_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 本文件验证自定义字段的三种场景（见 README 2.3 节），全部基于 campaign 表：
//   场景 A：普通自定义类型（无 serializer，自己实现 Value/Scan）
//   场景 B：json 类型（serializer: json，gorm 自动序列化）
//   场景 C：props 虚拟字段（数据库无列，业务代码手动赋值）
// 前三个方法分别独立验证，最后一个方法把三种场景合并在一起验证。

// TestOptionTypeCustom 验证场景 A：普通自定义类型（无 serializer，走自定义 Value/Scan）。
// 生成的 accessor 文件带 Value()/Scan() 空实现，需要自己填充。
func TestOptionTypeCustom(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE campaign (id INTEGER PRIMARY KEY, name TEXT, tags TEXT)`)

	yamlContent := `
relation:
  - table: campaign
    column:
      tags:
        type: CampaignTagsOption
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "campaign")

	// 字段类型为 *accessor.CampaignTagsOption，且无 serializer tag
	if !regexp.MustCompile(`Tags\s+\*accessor\.CampaignTagsOption`).MatchString(content) {
		t.Errorf("期望生成 *accessor.CampaignTagsOption 类型字段，实际：\n%s", content)
	}
	if strings.Contains(content, `serializer:`) {
		t.Errorf("普通自定义类型不应有 serializer tag，实际：\n%s", content)
	}
	// 生成的 accessor 文件带 Value()/Scan() 方法（common 模板）
	optionFile := filepath.Join(outDir, "accessor", "campaign_tags_option.go")
	data, err := os.ReadFile(optionFile)
	if err != nil {
		t.Fatalf("期望生成 accessor 类型文件 campaign_tags_option.go，实际未找到")
	}
	optionContent := string(data)
	if !strings.Contains(optionContent, "func (c CampaignTagsOption) Value()") {
		t.Errorf("普通自定义类型应带 Value() 方法，实际：\n%s", optionContent)
	}
	if !strings.Contains(optionContent, "func (c *CampaignTagsOption) Scan(") {
		t.Errorf("普通自定义类型应带 Scan() 方法，实际：\n%s", optionContent)
	}
}

// TestOptionTypeJSON 验证场景 B：json 类型（serializer: json，gorm 自动序列化）。
// 生成的 accessor 文件只有空 struct，无需实现 Value/Scan。
func TestOptionTypeJSON(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE campaign (id INTEGER PRIMARY KEY, name TEXT, content TEXT)`)

	yamlContent := `
relation:
  - table: campaign
    column:
      content:
        serializer: json
        type: CampaignContentOption
        tag:
          gorm:
            default: "''"
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "campaign")

	// 字段类型为 *accessor.CampaignContentOption，并带 serializer:json tag
	if !regexp.MustCompile(`Content\s+\*accessor\.CampaignContentOption`).MatchString(content) {
		t.Errorf("期望生成 *accessor.CampaignContentOption 类型字段，实际：\n%s", content)
	}
	if !strings.Contains(content, `serializer:json`) {
		t.Errorf("期望包含 serializer:json tag，实际：\n%s", content)
	}
	// 生成的 accessor 文件只有空 struct，无 Value/Scan（json 模板）
	optionFile := filepath.Join(outDir, "accessor", "campaign_content_option.go")
	data, err := os.ReadFile(optionFile)
	if err != nil {
		t.Fatalf("期望生成 accessor 类型文件 campaign_content_option.go，实际未找到")
	}
	optionContent := string(data)
	if strings.Contains(optionContent, "func (c CampaignContentOption) Value()") {
		t.Errorf("json 类型不应带 Value() 方法，实际：\n%s", optionContent)
	}
}

// TestOptionTypeProps 验证场景 C：props 虚拟字段（数据库无列，业务代码手动赋值）。
// 生成的字段只有 json tag，无 gorm tag（不参与数据库读写）。
func TestOptionTypeProps(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE campaign (id INTEGER PRIMARY KEY, name TEXT)`)

	yamlContent := `
config:
  tag_json_camel: lower
relation:
  - table: campaign
    props:
      user_count:
        type: CampaignUserCountOption
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "campaign")

	// 字段名为大驼峰，类型为 *accessor.XxxOption 指针
	if !regexp.MustCompile(`UserCount\s+\*accessor\.CampaignUserCountOption`).MatchString(content) {
		t.Errorf("期望生成 UserCount *accessor.CampaignUserCountOption 字段，实际：\n%s", content)
	}
	// json tag 受 tag_json_camel 控制（lower → 小写驼峰）
	if !regexp.MustCompile(`json:"userCount"`).MatchString(content) {
		t.Errorf("期望 json tag 为 userCount（小写驼峰），实际：\n%s", content)
	}
	// 虚拟字段无 gorm tag（不存在于数据库）
	if regexp.MustCompile(`UserCount\s+\*accessor\.CampaignUserCountOption\s+\x60gorm:`).MatchString(content) {
		t.Errorf("虚拟字段不应有 gorm tag，实际：\n%s", content)
	}
	// 生成对应的 accessor 类型文件
	optionFile := filepath.Join(outDir, "accessor", "campaign_user_count_option.go")
	if _, err := os.Stat(optionFile); err != nil {
		t.Errorf("期望生成 accessor 类型文件 campaign_user_count_option.go，实际未找到")
	}
}

// TestOptionTypeAll 验证三种场景合并：campaign 表同时包含
// 普通自定义类型（tags）、json 类型（content）、props 虚拟字段（user_count）。
func TestOptionTypeAll(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE campaign (id INTEGER PRIMARY KEY, name TEXT, content TEXT, tags TEXT)`)

	yamlContent := `
config:
  tag_json_camel: lower
relation:
  - table: campaign
    column:
      content:
        serializer: json
        type: CampaignContentOption
        tag:
          gorm:
            default: "''"
      tags:
        type: CampaignTagsOption
    props:
      user_count:
        type: CampaignUserCountOption
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "campaign")

	// 三个字段同时生成，类型各不同
	if !regexp.MustCompile(`Content\s+\*accessor\.CampaignContentOption`).MatchString(content) {
		t.Errorf("期望生成 Content *accessor.CampaignContentOption，实际：\n%s", content)
	}
	if !regexp.MustCompile(`Tags\s+\*accessor\.CampaignTagsOption`).MatchString(content) {
		t.Errorf("期望生成 Tags *accessor.CampaignTagsOption，实际：\n%s", content)
	}
	if !regexp.MustCompile(`UserCount\s+\*accessor\.CampaignUserCountOption`).MatchString(content) {
		t.Errorf("期望生成 UserCount *accessor.CampaignUserCountOption，实际：\n%s", content)
	}
	// content 有 serializer:json，tags 无 serializer
	if !strings.Contains(content, `serializer:json`) {
		t.Errorf("content 应带 serializer:json tag，实际：\n%s", content)
	}
	// user_count 只有 json tag，无 gorm tag
	if !regexp.MustCompile(`UserCount\s+\*accessor\.CampaignUserCountOption\s+\x60json:"userCount"\x60`).MatchString(content) {
		t.Errorf("user_count 应只有 json tag 无 gorm tag，实际：\n%s", content)
	}
	// 生成全部三个 accessor 类型文件
	for _, file := range []string{
		"campaign_content_option.go",
		"campaign_tags_option.go",
		"campaign_user_count_option.go",
	} {
		if _, err := os.Stat(filepath.Join(outDir, "accessor", file)); err != nil {
			t.Errorf("期望生成 accessor 类型文件 %s，实际未找到", file)
		}
	}
}
