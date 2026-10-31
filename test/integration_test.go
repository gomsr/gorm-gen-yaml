package yamlgen_test

import (
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	yamlgen "github.com/gomsr/gorm-gen-yaml/v2"
)

// integrationDB 基于仓库根目录的 test.db 和 gen.yaml，用于集成测试。
var integrationDB *gorm.DB

func init() {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)
	// test.db 位于仓库根目录（test/ 的上一级）
	path, _ := filepath.Abs(filepath.Join("..", "test.db"))
	integrationDB, _ = gorm.Open(sqlite.Open(path), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		Logger: newLogger,
	})
}

// TestParse 根据仓库内的 gen.yaml 生成所有 model 和 dao 文件。
// 这是对完整配置文件的集成验证。
func TestParse(t *testing.T) {
	os.RemoveAll("../output/dao")
	os.RemoveAll("../output/entity")

	g := gen.NewGenerator(gen.Config{
		OutPath:      "../output/dao",
		Mode:         gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface,
		ModelPkgPath: "entity",
	})
	g.UseDB(integrationDB)

	yamlgen.NewYamlGenerator("../gen.yaml").UseGormGenerator(g).Generate()

	g.Execute()
}
