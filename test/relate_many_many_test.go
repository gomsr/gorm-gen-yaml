package yamlgen_test

import (
	"regexp"
	"testing"
)

// TestRelateManyMany 验证 many_many 多对多关联。
// 场景：用户（user）与标签（tag）多对多，通过中间表 user_tag 给用户打标签。
func TestRelateManyMany(t *testing.T) {
	db := newTestDB(t,
		`CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE tag (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE user_tag (id INTEGER PRIMARY KEY, user_id INTEGER, tag_id INTEGER)`,
	)

	yamlContent := `
relation:
  - table: user
    relate:
      - table: tag
        many_2_many: user_tag
        foreign_key: id
        join_foreign_key: user_id
        join_references: tag_id
        references: id
        type: many_many
  - table: tag
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "user")

	// many_many 生成切片类型字段 []Tag，且包含 many2many 中间表 tag
	if !regexp.MustCompile(`Tag\s+\[\]Tag`).MatchString(content) {
		t.Errorf("期望 many_many 生成切片类型字段 Tag []Tag，实际：\n%s", content)
	}
	if !regexp.MustCompile(`many2many:user_tag`).MatchString(content) {
		t.Errorf("期望包含 many2many:user_tag tag，实际：\n%s", content)
	}
}

// TestRelateManyManyMultiple 验证同一张表多次多对多关联，通过 name 区分字段名。
// 场景：邮件活动既包含某些标签（IncludeTags）又排除某些标签（ExcludeTags），
// 两次都关联 tag 表，但用不同的中间表，靠 name 区分。
func TestRelateManyManyMultiple(t *testing.T) {
	db := newTestDB(t,
		`CREATE TABLE campaign (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE tag (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE campaign_tag_include (id INTEGER PRIMARY KEY, campaign_id INTEGER, tag_id INTEGER)`,
		`CREATE TABLE campaign_tag_exclude (id INTEGER PRIMARY KEY, campaign_id INTEGER, tag_id INTEGER)`,
	)

	yamlContent := `
relation:
  - table: campaign
    relate:
      # 包含的标签：下划线写法，转成 IncludeTags
      - table: tag
        name: include_tags
        many_2_many: campaign_tag_include
        foreign_key: id
        join_foreign_key: campaign_id
        join_references: tag_id
        references: id
        type: many_many
      # 排除的标签：驼峰写法，原样保留 ExcludeTags
      - table: tag
        name: ExcludeTags
        many_2_many: campaign_tag_exclude
        foreign_key: id
        join_foreign_key: campaign_id
        join_references: tag_id
        references: id
        type: many_many
  - table: tag
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "campaign")

	// name 为下划线写法，应转为大驼峰 IncludeTags
	if !regexp.MustCompile(`IncludeTags\s+\[\]Tag`).MatchString(content) {
		t.Errorf("期望 include_tags 转为字段名 IncludeTags，实际：\n%s", content)
	}
	// name 为驼峰写法，应原样保留 ExcludeTags
	if !regexp.MustCompile(`ExcludeTags\s+\[\]Tag`).MatchString(content) {
		t.Errorf("期望 ExcludeTags 原样保留，实际：\n%s", content)
	}
	// 两个关联分别指向不同的中间表
	if !regexp.MustCompile(`many2many:campaign_tag_include`).MatchString(content) {
		t.Errorf("期望包含 many2many:campaign_tag_include tag，实际：\n%s", content)
	}
	if !regexp.MustCompile(`many2many:campaign_tag_exclude`).MatchString(content) {
		t.Errorf("期望包含 many2many:campaign_tag_exclude tag，实际：\n%s", content)
	}
}
