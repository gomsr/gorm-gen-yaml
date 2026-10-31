package yamlgen_test

import (
	"regexp"
	"testing"
)

// TestRelateHasOne 验证 has_one 一对一关联。
// 场景：每个邮件活动（campaign）有一份发送配置（campaign_config）。
func TestRelateHasOne(t *testing.T) {
	db := newTestDB(t,
		`CREATE TABLE campaign (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE campaign_config (id INTEGER PRIMARY KEY, campaign_id INTEGER, subject TEXT)`,
	)

	yamlContent := `
relation:
  - table: campaign
    relate:
      - table: campaign_config
        foreign_key: campaign_id
        type: has_one
  - table: campaign_config
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "campaign")

	// has_one 生成指针类型字段 *CampaignConfig
	if !regexp.MustCompile(`CampaignConfig\s+\*CampaignConfig`).MatchString(content) {
		t.Errorf("期望 has_one 生成指针类型字段 *CampaignConfig，实际：\n%s", content)
	}
}
