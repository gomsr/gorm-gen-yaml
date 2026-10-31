package yamlgen_test

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	yamlgen "github.com/gomsr/gorm-gen-yaml/v2"
)

// 本文件提供各测试用例共用的工具函数。
// 具体测试用例按场景拆分到 *_test.go 文件中：
//   - config_test.go            全局配置与字段配置
//   - audit_field_test.go       审计字段的两种配置方式
//   - relate_has_one_test.go    一对一关联
//   - relate_has_many_test.go   一对多关联
//   - relate_belongs_to_test.go 从属关联
//   - relate_many_many_test.go  多对多关联（含同一张表多次关联）
//   - integration_test.go       基于 gen.yaml + test.db 的完整集成验证

// newTestDB 创建一个使用临时 sqlite 数据库的 gorm.DB，
// 并执行给定的建表语句，供各测试用例独立使用（互不干扰）。
func newTestDB(t *testing.T, createSQL ...string) *gorm.DB {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开 sqlite 失败: %v", err)
	}
	for _, sql := range createSQL {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("建表失败: %v\nsql: %s", err, sql)
		}
	}
	return db
}

// generate 使用给定的 yaml 内容执行代码生成，返回输出目录（含 entity / dao / accessor）。
// 注意：输出目录放在项目模块内（而非系统临时目录），
// 因为自定义 Option 类型需要通过 packages.Load 解析 accessor 包的 import 路径，
// 这要求 accessor 目录位于当前 go module 内。
func generate(t *testing.T, db *gorm.DB, yamlContent string) string {
	t.Helper()

	// 在项目根目录下创建临时输出目录（测试结束后清理）
	baseDir := filepath.Join("..", ".tmp_test")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("创建输出目录失败: %v", err)
	}
	outDir, err := os.MkdirTemp(baseDir, "out-*")
	if err != nil {
		t.Fatalf("创建输出目录失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(baseDir) })

	g := gen.NewGenerator(gen.Config{
		OutPath:      outDir + "/dao",
		Mode:         gen.WithoutContext,
		ModelPkgPath: "entity",
	})
	g.UseDB(db)

	yamlPath := filepath.Join(t.TempDir(), "gen.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("写入 yaml 失败: %v", err)
	}

	yamlgen.NewYamlGenerator(yamlPath).UseGormGenerator(g).Generate()
	g.Execute()

	return outDir
}

// readEntity 读取生成的 entity 文件内容。
func readEntity(t *testing.T, outDir, tableName string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(outDir, "entity", tableName+".gen.go"))
	if err != nil {
		t.Fatalf("读取生成的 entity 文件失败: %v", err)
	}
	return string(data)
}
