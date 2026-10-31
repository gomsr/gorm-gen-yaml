package yamlgen_test

import (
	"regexp"
	"testing"
)

// TestAuditFieldAutoTimeTag 验证方式 B：用 autoCreateTime/autoUpdateTime tag
// 让任意字段名（非 CreatedAt/UpdatedAt）也能自动填充时间。
// 这是 gorm 原生支持的机制，不依赖字段名。
func TestAuditFieldAutoTimeTag(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE demo_user (id INTEGER PRIMARY KEY, name TEXT, create_time DATETIME, update_time DATETIME, delete_time DATETIME)`)

	yamlContent := `
relation:
  - table: demo_user
    column:
      create_time:
        type: time.Time
        tag:
          gorm:
            autoCreateTime: ""
      update_time:
        type: time.Time
        tag:
          gorm:
            autoUpdateTime: ""
            type: datetime(3)
      delete_time:
        type: gorm.DeletedAt
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "demo_user")

	// 字段名 CreateTime（非 CreatedAt），但通过 autoCreateTime tag 触发自动填充
	if !regexp.MustCompile(`CreateTime\s+time\.Time`).MatchString(content) {
		t.Errorf("期望生成 CreateTime time.Time 字段，实际：\n%s", content)
	}
	if !regexp.MustCompile(`autoCreateTime`).MatchString(content) {
		t.Errorf("期望包含 autoCreateTime tag，实际：\n%s", content)
	}
	if !regexp.MustCompile(`autoUpdateTime`).MatchString(content) {
		t.Errorf("期望包含 autoUpdateTime tag，实际：\n%s", content)
	}
	// 额外的列类型覆盖
	if !regexp.MustCompile(`datetime\(3\)`).MatchString(content) {
		t.Errorf("期望包含 datetime(3) 列类型 tag，实际：\n%s", content)
	}
	// 软删除字段
	if !regexp.MustCompile(`DeleteTime\s+gorm\.DeletedAt`).MatchString(content) {
		t.Errorf("期望生成 DeleteTime gorm.DeletedAt 字段，实际：\n%s", content)
	}
}

// TestAuditFieldByFieldName 验证方式 A：字段名 created_at/updated_at 自动识别，
// 无需任何 tag。用于和方式 B 对照。
func TestAuditFieldByFieldName(t *testing.T) {
	db := newTestDB(t, `CREATE TABLE demo_user (id INTEGER PRIMARY KEY, name TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`)

	yamlContent := `
relation:
  - table: demo_user
    column:
      deleted_at:
        type: gorm.DeletedAt
`
	outDir := generate(t, db, yamlContent)
	content := readEntity(t, outDir, "demo_user")

	// 字段名 CreatedAt/UpdatedAt，gorm 自动识别为 time.Time
	if !regexp.MustCompile(`CreatedAt\s+time\.Time`).MatchString(content) {
		t.Errorf("期望生成 CreatedAt time.Time 字段，实际：\n%s", content)
	}
	if !regexp.MustCompile(`UpdatedAt\s+time\.Time`).MatchString(content) {
		t.Errorf("期望生成 UpdatedAt time.Time 字段，实际：\n%s", content)
	}
	if !regexp.MustCompile(`DeletedAt\s+gorm\.DeletedAt`).MatchString(content) {
		t.Errorf("期望生成 DeletedAt gorm.DeletedAt 字段，实际：\n%s", content)
	}
}
