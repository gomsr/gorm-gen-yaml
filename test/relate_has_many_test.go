package yamlgen_test

import (
	"regexp"
	"testing"
)

// TestRelateHasMany 验证 has_many 一对多关联。
// 场景：一个用户（user）有多条活动参与历史（user_campaign_log）。
func TestRelateHasMany(t *testing.T) {
	db := newTestDB(t,
		`CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE user_campaign_log (id INTEGER PRIMARY KEY, user_id INTEGER, action TEXT)`,
	)

	yamlContent := `
relation:
  - table: user
    relate:
      - table: user_campaign_log
        foreign_key: user_id
        type: has_many
  - table: user_campaign_log
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "user")

	// has_many 生成切片类型字段 []UserCampaignLog
	if !regexp.MustCompile(`UserCampaignLog\s+\[\]UserCampaignLog`).MatchString(content) {
		t.Errorf("期望 has_many 生成切片类型字段 []UserCampaignLog，实际：\n%s", content)
	}
}

// TestRelateHasManyMultiple 验证 has_many 同一张表多次关联，通过 name 区分字段名。
// 场景：一个用户（user）既「发送」也「接收」消息（message），
// 两次都 has_many 关联 message 表，用不同外键 + name 区分。
func TestRelateHasManyMultiple(t *testing.T) {
	db := newTestDB(t,
		`CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE message (id INTEGER PRIMARY KEY, sender_id INTEGER, receiver_id INTEGER, content TEXT)`,
	)

	yamlContent := `
relation:
  - table: user
    relate:
      # 发送的消息：下划线写法，转成 SentMessages
      - table: message
        name: sent_messages
        foreign_key: sender_id
        type: has_many
      # 接收的消息：驼峰写法，原样保留 ReceivedMessages
      - table: message
        name: ReceivedMessages
        foreign_key: receiver_id
        type: has_many
  - table: message
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "user")

	// name 为下划线写法，应转为大驼峰 SentMessages
	if !regexp.MustCompile(`SentMessages\s+\[\]Message`).MatchString(content) {
		t.Errorf("期望 sent_messages 转为字段名 SentMessages，实际：\n%s", content)
	}
	// name 为驼峰写法，应原样保留 ReceivedMessages
	if !regexp.MustCompile(`ReceivedMessages\s+\[\]Message`).MatchString(content) {
		t.Errorf("期望 ReceivedMessages 原样保留，实际：\n%s", content)
	}
	// 两个关联分别用不同的外键
	if !regexp.MustCompile(`foreignKey:sender_id`).MatchString(content) {
		t.Errorf("期望包含 foreignKey:sender_id tag，实际：\n%s", content)
	}
	if !regexp.MustCompile(`foreignKey:receiver_id`).MatchString(content) {
		t.Errorf("期望包含 foreignKey:receiver_id tag，实际：\n%s", content)
	}
}
