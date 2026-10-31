package yamlgen_test

import (
	"regexp"
	"testing"
)

// TestRelateBelongsTo 验证 belongs_to 从属关联。
// 场景：每条活动参与历史（user_campaign_log）属于一个用户（user）和一个活动（campaign）。
func TestRelateBelongsTo(t *testing.T) {
	db := newTestDB(t,
		`CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE campaign (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE user_campaign_log (id INTEGER PRIMARY KEY, user_id INTEGER, campaign_id INTEGER, action TEXT)`,
	)

	yamlContent := `
relation:
  - table: user_campaign_log
    relate:
      - table: user
        foreign_key: user_id
        references: id
        type: belongs_to
      - table: campaign
        foreign_key: campaign_id
        references: id
        type: belongs_to
  - table: user
  - table: campaign
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "user_campaign_log")

	// belongs_to 生成指针类型字段 *User 和 *Campaign
	if !regexp.MustCompile(`User\s+\*User`).MatchString(content) {
		t.Errorf("期望 belongs_to 生成指针类型字段 *User，实际：\n%s", content)
	}
	if !regexp.MustCompile(`Campaign\s+\*Campaign`).MatchString(content) {
		t.Errorf("期望 belongs_to 生成指针类型字段 *Campaign，实际：\n%s", content)
	}
}
