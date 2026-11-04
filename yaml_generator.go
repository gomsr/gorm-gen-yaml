package yamlgen

import (
	"errors"
	"fmt"
	"github.com/gomsr/gorm-gen-yaml/template"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v3"
	"gorm.io/gen"
	"gorm.io/gen/field"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

type GeneratorTable struct {
	Flag      uint
	ModelName string
}

type YamlGenerator struct {
	yaml                *DbTable
	gen                 *gen.Generator
	generatedTable      map[string]*GeneratorTable
	inProgress          map[string]bool
	columnOptionSaveDir string
}

func NewYamlGenerator(path string) *YamlGenerator {
	obj := &YamlGenerator{}
	err := obj.loadFromFile(path)
	if err != nil {
		panic(err)
		return nil
	}
	obj.generatedTable = make(map[string]*GeneratorTable)
	obj.inProgress = make(map[string]bool)
	return obj
}

type DbTable struct {
	Config   Config  `yaml:"config"`
	Table    []Table `yaml:"relation"`
	TableMap map[string]*Table

	// RelateTableMap 记录「被任何表引用过」的表名。
	//
	// Deprecated: 环检测请使用 inProgress。该集合只能表达“入度 > 0”，
	// 无法区分「被引用过」与「在当前递归路径上」，按它判断是否递归会把
	// 大量非环场景误判为环，进而生成缺少关联的裸模型，导致产物随遍历顺序变化。
	// 字段暂时保留以兼容外部引用，后续版本移除。
	RelateTableMap map[string]struct{}
}

type Table struct {
	Name   string            `yaml:"table"`
	Relate []Relate          `yaml:"relate"`
	Column map[string]Column `yaml:"column"`
	Props  map[string]Column `yaml:"props"`
}

type Column struct {
	Type       string                       `yaml:"type"`
	Serializer string                       `yaml:"serializer"`
	Tag        map[string]map[string]string `yaml:"tag"`
	Comment    string                       `yaml:"comment"`
	Rename     string                       `yaml:"rename"`
}

type Relate struct {
	Table          string `yaml:"table"`
	Name           string `yaml:"name"`
	ForeignKey     string `yaml:"foreign_key"`
	References     string `yaml:"references"`
	JoinForeignKey string `yaml:"join_foreign_key"`
	JoinReferences string `yaml:"join_references"`
	Many2many      string `yaml:"many_2_many"`
	Type           string `yaml:"type"`
}

type Config struct {
	TagJsonCamel string `yaml:"tag_json_camel"`
}

func (y *YamlGenerator) UseGormGenerator(g *gen.Generator) *YamlGenerator {
	y.gen = g

	y.SetColumnOptionSaveDir(g.Config.OutPath + "/../accessor")

	return y
}

func (y *YamlGenerator) loadFromFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err != nil {
		return errors.New(fmt.Sprintf("%s file not found", path))
	}
	content, _ := io.ReadAll(file)
	y.yaml = &DbTable{}
	err = yaml.Unmarshal(content, y.yaml)
	if err != nil {
		return err
	}
	y.yaml.TableMap = make(map[string]*Table)
	// Deprecated: 仅为兼容保留，生成流程已不再读取；环检测见 generateFromTable 的 inProgress。
	y.yaml.RelateTableMap = make(map[string]struct{})

	for _, table := range y.yaml.Table {
		t := table
		y.yaml.TableMap[table.Name] = &t

		for _, relateTable := range table.Relate {
			y.yaml.RelateTableMap[relateTable.Table] = struct{}{}
		}
	}

	return nil
}

func (y *YamlGenerator) SetColumnOptionSaveDir(columnOptionSaveDir string) {
	y.columnOptionSaveDir = columnOptionSaveDir

	if err := os.MkdirAll(y.columnOptionSaveDir, os.ModePerm); err != nil {
		panic(err)
	}
}

func (y *YamlGenerator) generateColumnOption(column Column) error {
	var columnOptionTemplate string
	var exists bool

	if column.Serializer == "json" || column.Serializer == "gob" || column.Serializer == "unixtime" {
		columnOptionTemplate, exists = template.ColumnOptionTemplate["json"]
	} else {
		columnOptionTemplate, exists = template.ColumnOptionTemplate["common"]
	}

	if !exists {
		return errors.New("serializer type not support")
	}
	columnOptionTemplate = strings.Replace(columnOptionTemplate, "{{Package}}", strings.TrimRight(path.Base(y.columnOptionSaveDir), "/"), 1)
	columnOptionTemplate = strings.Replace(columnOptionTemplate, "{{OptionStructName}}", column.Type, -1)

	p := y.columnOptionSaveDir + "/" + CamelCaseToUnderscore(column.Type) + ".go"
	_, err := os.Stat(p)
	if os.IsNotExist(err) {
		return os.WriteFile(p, []byte(columnOptionTemplate), 0640)
	}
	return nil
}

func (y *YamlGenerator) getTableRelateOpt(table *Table) []gen.ModelOpt {
	opt := make([]gen.ModelOpt, len(table.Relate))
	for i, table := range table.Relate {
		relatePointer := false
		var fieldType field.RelationshipType
		switch table.Type {
		case "has_one":
			fieldType = field.HasOne
			relatePointer = true
		case "has_many":
			fieldType = field.HasMany
		case "many_many":
			fieldType = field.Many2Many
		case "belongs_to":
			fieldType = field.BelongsTo
			relatePointer = true
		}
		relateConfig := make(field.GormTag)

		if table.ForeignKey != "" {
			relateConfig.Append("foreignKey", table.ForeignKey)
		}
		if table.JoinForeignKey != "" {
			relateConfig.Append("joinForeignKey", table.JoinForeignKey)
		}
		if table.References != "" {
			relateConfig.Append("references", table.References)
		}
		if table.JoinReferences != "" {
			relateConfig.Append("joinReferences", table.JoinReferences)
		}
		if table.Many2many != "" {
			relateConfig.Append("many2many", table.Many2many)
		}

		// 关联字段名：优先使用 name（支持同一张表多次关联，通过 name 区分字段名），
		// name 为空时回退到目标 model 结构体名，保持向后兼容
		relateFieldName := table.Name
		if relateFieldName == "" {
			relateFieldName = y.generatedTable[table.Table].ModelName
		} else if strings.Contains(relateFieldName, "_") {
			// 仅当下划线命名时转为大驼峰；已是驼峰的原样保留
			relateFieldName = UnderscoreToCamelCase(relateFieldName, true)
		}

		relateMeta, ok := y.generatedTable[table.Table]
		if !ok {
			panic(fmt.Errorf("relation table <%s> of <%s> is not generated yet", table.Table, table.Name))
		}
		genInfo := y.gen.Data[relateMeta.ModelName]
		if genInfo == nil {
			panic(fmt.Errorf("relation table <%s> has no model info, model name: %s", table.Table, relateMeta.ModelName))
		}

		opt[i] = gen.FieldRelate(fieldType, relateFieldName, genInfo.QueryStructMeta, &field.RelateConfig{
			GORMTag:       relateConfig,
			RelatePointer: relatePointer,
		})
	}

	return opt
}

func (y *YamlGenerator) getTableColumnOpt(table *Table) ([]gen.ModelOpt, bool) {
	opt := make([]gen.ModelOpt, 0)
	//找到column生成自定义column类型
	hasOption := false
	// 按列名有序遍历，保证同一份 yaml 多次生成结果一致（map 遍历顺序是随机的）
	for _, name := range sortedKeys(table.Column) {
		column := table.Column[name]
		if column.Type != "" {
			if strings.Contains(strings.ToLower(column.Type), "option") {
				if column.Serializer == "json" || column.Serializer == "gob" || column.Serializer == "unixtime" {
					if column.Tag == nil {
						column.Tag = map[string]map[string]string{
							"gorm": {
								"serializer": column.Serializer,
							},
						}
					} else {
						column.Tag["gorm"]["serializer"] = column.Serializer
					}
					// 生成对应的类型文件
					err := y.generateColumnOption(column)
					if err != nil {
						panic(err)
					}
				} else {
					// 自定义生成 Scan Value
					err := y.generateColumnOption(column)
					if err != nil {
						panic(err)
					}
				}
				hasOption = true
				opt = append(opt, gen.FieldType(name, "*"+strings.TrimRight(path.Base(y.columnOptionSaveDir), "/")+"."+column.Type))
			} else {
				opt = append(opt, gen.FieldType(name, column.Type))
			}
		}
		if column.Tag != nil {
			for _, tagType := range sortedKeys(column.Tag) {
				ttags := column.Tag[tagType]
				if tagType == "gorm" {
					opt = append(opt, gen.FieldGORMTag(name, func(tag field.GormTag) field.GormTag {
						for _, tagName := range sortedKeys(ttags) {
							tag = tag.Set(tagName, ttags[tagName])
						}
						return tag
					}))
				} else {
					tTagType := tagType
					opt = append(opt, gen.FieldTag(name, func(tag field.Tag) field.Tag {
						tagStr := ""
						for _, tagName := range sortedKeys(ttags) {
							if val := ttags[tagName]; val == "" {
								tagStr += tagName + ";"
							} else {
								tagStr += tagName + ":" + val + ";"
							}
						}
						return tag.Set(tTagType, strings.TrimRight(tagStr, ";"))
					}))
				}
			}
		}
		if column.Rename != "" {
			opt = append(opt, gen.FieldRename(name, column.Rename))
		}
		if column.Comment != "" {
			opt = append(opt, gen.FieldComment(name, column.Comment))
		}
	}

	for _, name := range sortedKeys(table.Props) {
		column := table.Props[name]
		tag := field.Tag{}
		tag.Set(field.TagKeyJson, UnderscoreToCamelCase(name, y.yaml.Config.TagJsonCamel == "upper"))
		opt = append(opt, gen.FieldNew(UnderscoreToCamelCase(name, true), "*"+strings.TrimRight(path.Base(y.columnOptionSaveDir), "/")+"."+column.Type, tag))
		// 自定义生成 Scan Value
		column.Serializer = "common"
		err := y.generateColumnOption(column)
		if err != nil {
			panic(err)
		}
		hasOption = true
	}
	return opt, hasOption
}

// generateFromTable 生成单表模型。
//
// 注意：gen.FieldRelate 在调用时会把目标表“当时”的 Relations() 快照进字段里，
// 所以生成顺序会直接影响关联字段的嵌套深度。这里先递归把关联目标完整生成，
// 再用 inProgress 兜住双向/环状关联，保证同一份 yaml 的产出只由 yaml 内容决定。
func (y *YamlGenerator) generateFromTable(table *Table, opt ...gen.ModelOpt) {
	if table == nil {
		return
	}
	if generatedTable, exists := y.generatedTable[table.Name]; exists && generatedTable.Flag == 2 {
		return
	}
	// 环：A→B→A 时 B 处理到 A，直接返回；A 的基础模型已由 ensureBaseModel 注册
	if y.inProgress[table.Name] {
		return
	}
	y.inProgress[table.Name] = true
	defer delete(y.inProgress, table.Name)

	for _, relate := range table.Relate {
		// 目标表在 yaml 顶层声明时，优先完整生成它（含它自己的 relate/column 定制），
		// 避免它先以“裸模型”出现，导致父表拿到的快照缺少二级关联。
		if target, inTable := y.yaml.TableMap[relate.Table]; inTable && !y.inProgress[relate.Table] {
			y.generateFromTable(target, opt...)
		}
		// 环上的表 / 只出现在 relate 中的表：兜底生成一个基础模型，保证有元数据可用
		y.ensureBaseModel(relate.Table, opt...)
	}

	//找到所有relate,生成模型
	relateOpt := y.getTableRelateOpt(table)
	columnOpt, hasOption := y.getTableColumnOpt(table)
	if opt == nil {
		opt = make([]gen.ModelOpt, 0)
	}
	opt = append(opt, relateOpt...)
	opt = append(opt, columnOpt...)
	relateMate := y.gen.GenerateModel(table.Name, opt...)
	if hasOption {
		pkgs, err := packages.Load(&packages.Config{
			Mode: packages.NeedName,
			Dir:  y.columnOptionSaveDir,
		})
		if err != nil {
			panic(err)
		}
		relateMate.ImportPkgPaths = append(relateMate.ImportPkgPaths, "\""+pkgs[0].PkgPath+"\"")
	}
	if _, exists := y.generatedTable[table.Name]; exists {
		delete(y.gen.Data, y.generatedTable[table.Name].ModelName)
	}
	y.gen.ApplyBasic(relateMate)
	y.generatedTable[table.Name] = &GeneratorTable{
		Flag:      2,
		ModelName: relateMate.ModelStructName,
	}
}

// ensureBaseModel 为目标表生成一个不含关联/列定制的基础模型（已存在则跳过）。
// 用于环状关联中的回边，以及只出现在 relate 里、没有在 yaml 顶层声明的表。
func (y *YamlGenerator) ensureBaseModel(tableName string, opt ...gen.ModelOpt) {
	if _, exists := y.generatedTable[tableName]; exists {
		return
	}
	mate := y.gen.GenerateModel(tableName, opt...)
	if mate == nil {
		return
	}
	y.gen.ApplyBasic(mate)

	flag := uint(2)
	if _, inTable := y.yaml.TableMap[tableName]; inTable {
		flag = 1 // 之后还会被完整生成一次
	}
	y.generatedTable[tableName] = &GeneratorTable{
		Flag:      flag,
		ModelName: mate.ModelStructName,
	}
}

// sortedKeys 返回 map 的有序 key：Go 的 map 遍历顺序随机，
// 直接 range map 会让同一份 yaml 每次生成出不同的代码。
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (y *YamlGenerator) Generate(opt ...gen.ModelOpt) {
	if y.yaml.Config.TagJsonCamel != "" {
		y.gen.WithJSONTagNameStrategy(func(columnName string) (tagContent string) {
			return UnderscoreToCamelCase(columnName, y.yaml.Config.TagJsonCamel == "upper")
		})
	}
	// 固定按表名遍历，保证多次生成结果可复现
	for _, name := range sortedKeys(y.yaml.TableMap) {
		y.generateFromTable(y.yaml.TableMap[name], opt...)
	}
}
