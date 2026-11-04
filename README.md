# gorm-gen-yaml

根据 yaml 配置文件生成数据库表的 model 文件及 dao 文件。

程序会根据配置文件中声明的表之间的依赖关系，按照顺序进行生成，让你摆脱复杂的关联关系管理。

# 快速开始

```go
g := gen.NewGenerator(gen.Config{
    OutPath:      "./output/dao",
    Mode:         gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface, // generate mode
    ModelPkgPath: "entity",
})
g.UseDB(db)
yamlgen.NewYamlGenerator("./gen.yaml").UseGormGenerator(g).Generate()
g.Execute()
```

生成产物：

- `output/entity` — model 文件
- `output/dao` — dao 查询文件
- `output/accessor` — 自定义类型（Option）文件

---

# 示例场景

本文档使用一个**邮件营销系统**作为示例，覆盖 gorm 的全部关联类型。

示例数据库由 [schema.sql](./schema.sql) 生成（sqlite3）：

```bash
sqlite3 test.db < schema.sql
```

## 表结构

### 业务表（含审计字段）

| 表 | 说明 |
|----|------|
| `user` | 用户 |
| `tag` | 标签（用于圈选/排除目标用户） |
| `campaign` | 邮件活动 |
| `user_campaign_log` | 用户活动参与历史 |
| `campaign_config` | 活动发送配置（一对一） |

### 中间表（关联表，不含审计字段）

| 表 | 说明 |
|----|------|
| `user_tag` | 用户-标签（多对多） |
| `campaign_tag_include` | 活动-包含标签（多对多） |
| `campaign_tag_exclude` | 活动-排除标签（多对多） |

### 关联关系总览

| 关联 | 类型 | 场景 |
|------|------|------|
| `user` ↔ `tag` | many_many | 用户打标签（`user_tag`） |
| `campaign` ↔ `tag` ×2 | many_many | 包含标签 / 排除标签（同一张表多次关联，靠 `name` 区分） |
| `user` → `user_campaign_log` | has_many | 一个用户有多条参与历史 |
| `user_campaign_log` → `user` | belongs_to | 记录属于某个用户 |
| `user_campaign_log` → `campaign` | belongs_to | 记录对应某个活动 |
| `campaign` → `campaign_config` | has_one | 一份发送配置 |

---

# 配置文件说明

完整示例见 [gen.yaml](./gen.yaml)。

## 顶层结构

```yaml
config:      # 全局配置（可选）
  tag_json_camel: lower
relation:    # 表及关联关系配置
  - table: campaign
    column: {...}
    props: {...}
    relate: [...]
```

## 1. config 全局配置

| 字段 | 取值 | 说明 |
|------|------|------|
| `tag_json_camel` | `lower` / `upper` / 不填 | 控制**所有字段** `json` tag 的命名风格 |

`tag_json_camel` 对生成结果的影响（以数据库字段 `user_name` 为例）：

| 取值 | 生成的 `json` tag |
|------|-------------------|
| 不填（默认） | `json:"user_name"`（保持下划线） |
| `lower` | `json:"userName"`（首字母小写驼峰） |
| `upper` | `json:"UserName"`（首字母大写驼峰） |

> 注意：该配置只影响 `json` tag，**不影响** Go 结构体字段名（字段名始终是大驼峰 `UserName`），也不影响 `gorm` tag（始终是数据库原字段名 `column:user_name`）。

## 2. relation 表配置

`relation` 是一个数组，每个元素对应一张表。`table` 字段指定表名，其余字段（`column`、`props`、`relate`）均为可选。

### 2.1 column — 字段配置

`column` 是一个 map，key 是数据库字段名，value 用于覆盖或增强该字段的生成。所有子项均可选：

```yaml
column:
  status:
    type: StatusOption      # 覆盖字段的 Go 类型
    serializer: json        # 序列化方式
    comment: 状态           # 字段注释
    rename: Status          # 重命名字段
    tag:                    # 自定义 struct tag
      gorm:
        default: "'active'"
      json:
        omitempty: ""
```

各子项说明：

| 子项 | 作用 | 示例 |
|------|------|------|
| `type` | 覆盖字段的 Go 类型 | `type: int64`、`type: StatusOption`、`type: gorm.DeletedAt` |
| `serializer` | 序列化方式，取值 `json` / `gob` / `unixtime` | `serializer: json` |
| `comment` | 生成字段的注释 | `comment: 用户状态` |
| `rename` | 重命名字段（改结构体字段名，不改 db 列名） | `rename: DeletedAt` |
| `tag` | 自定义 struct tag，格式为 `标签名: {key: value}` | 见上例 |

> 关于自定义类型（`type` 含 `option` 字样）的详细用法，见 [2.3 自定义类型与虚拟字段](#23-自定义类型与虚拟字段)。

### 2.2 审计字段（自动填充时间 + 软删除）

业务表通常需要审计字段。gorm 对时间字段的**自动填充**和**软删除**有原生支持，有两种配置方式。

#### 方式 A：靠字段名自动识别（最简单）

gorm 会**自动识别固定字段名** `CreatedAt` / `UpdatedAt`，无需任何额外配置：

| 字段名 | Go 类型 | 行为 |
|--------|---------|------|
| `created_at` | `time.Time` | 创建时自动填充当前时间 |
| `updated_at` | `time.Time` | 创建/更新时自动填充当前时间 |
| `deleted_at` | `gorm.DeletedAt` | 软删除标记，删除时自动填充时间 |

```yaml
column:
  deleted_at:
    type: gorm.DeletedAt   # created_at/updated_at 不用写，字段名自动识别
```

生成的 model：

```go
type Campaign struct {
    // ...
    CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
    UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
    DeletedAt gorm.DeletedAt `gorm:"column:deleted_at" json:"deletedAt"`
}
```

> `CreatedAt`/`UpdatedAt` 只要字段名匹配，gorm 会自动识别类型并填充。`DeletedAt` 必须指定 `type: gorm.DeletedAt` 才能获得软删除能力。

#### 方式 B：靠 tag 指定（任意字段名）

如果你的团队习惯用 `create_time` / `update_time` / `delete_time` 这类命名（而不是 `created_at`），可以用 gorm 原生的 `autoCreateTime` / `autoUpdateTime` tag 显式指定，**不依赖字段名**：

```yaml
column:
  create_time:
    type: time.Time
    tag:
      gorm:
        autoCreateTime: ""   # 空值 → 生成无值 tag autoCreateTime
  update_time:
    type: time.Time
    tag:
      gorm:
        autoUpdateTime: ""
  delete_time:
    type: gorm.DeletedAt
```

生成的 model：

```go
type User struct {
    // ...
    CreateTime time.Time      `gorm:"column:create_time;autoCreateTime" json:"createTime"`
    UpdateTime time.Time      `gorm:"column:update_time;autoUpdateTime" json:"updateTime"`
    DeleteTime gorm.DeletedAt `gorm:"column:delete_time" json:"deleteTime"`
}
```

> `autoCreateTime` / `autoUpdateTime` 是 gorm 内置 tag，字段名可以随意（`CreateTime`、`CreateTs` 等都行），gorm 运行时会根据 tag 自动填充时间。**无需修改任何生成器代码**，只需在 yaml 里写 tag。

#### 两种方式对比

| | 方式 A | 方式 B |
|---|--------|--------|
| 触发机制 | 字段名 `CreatedAt`/`UpdatedAt` | tag `autoCreateTime`/`autoUpdateTime` |
| 字段名要求 | 必须是固定名字 | 任意 |
| 配置复杂度 | 最简（`deleted_at` 除外都不用写） | 需写 tag |
| 适用场景 | 新项目、无命名约束 | 已有 `create_time` 等命名习惯的团队 |

本示例中 `user` 表使用方式 B，其余表使用方式 A，两种写法对照参考。



### 2.3 自定义类型与虚拟字段

当 `column` 或 `props` 里的 `type` 含 `option` 字样（不区分大小写）时，会当作**自定义类型**处理：

1. 在 `output/accessor` 目录生成对应的类型文件（如 `StatusOption` → `status_option.go`）
2. 字段类型变为 `*accessor.StatusOption`（指针）
3. 生成的 accessor 文件是**模板骨架**，需要你手动填充字段和逻辑

自定义类型根据「数据库是否有列」和「是否用 serializer」分为三种场景：

| 场景 | 数据库有列 | serializer | 你要写什么 |
|------|:---:|:---:|----------|
| A. 普通自定义类型 | ✅ | 无 | struct 字段 + 实现 `Value()`/`Scan()` |
| B. json 类型 | ✅ | `json` | 只写 struct 字段（gorm 自动序列化） |
| C. props 虚拟字段 | ❌ | 无 | struct 字段 + 业务代码手动赋值 |

#### 场景 A：普通自定义类型（存非 JSON 格式，自己写 Value/Scan）

数据库存的是自定义格式（如逗号分隔字符串），你希望 Go 里用更友好的类型（如 `[]string`）表达。

以 `campaign.tags`（存 `"vip,new"` 逗号分隔）为例：

```yaml
column:
  tags:
    type: CampaignTagsOption   # 无 serializer
```

生成的 accessor 文件（带 `Value()`/`Scan()` 空实现，你要填充）：

```go
package accessor

import (
    "database/sql/driver"
    "strings"
)

type CampaignTagsOption struct {
    Tags []string   // 你加的字段
}

// 写库：[]string → "vip,new"
func (c CampaignTagsOption) Value() (driver.Value, error) {
    return strings.Join(c.Tags, ","), nil
}

// 读库："vip,new" → []string
func (c *CampaignTagsOption) Scan(value interface{}) error {
    if v, ok := value.(string); ok {
        c.Tags = strings.Split(v, ",")
    }
    return nil
}
```

生成的 model：

```go
Tags *accessor.CampaignTagsOption `gorm:"column:tags;type:TEXT" json:"tags"`
```

#### 场景 B：json 类型（存 JSON，gorm 自动序列化）

数据库存 JSON 字符串，gorm 用内置 json 序列化器自动读写。以 `campaign.content` 为例：

```yaml
column:
  content:
    serializer: json              # 走 gorm 内置 json 序列化器
    type: CampaignContentOption   # 类型以 Option 结尾
    tag:
      gorm:
        default: "''"
```

生成的 accessor 文件（**只有空 struct，你只需加字段，无需写 Value/Scan**）：

```go
package accessor

type CampaignContentOption struct {
    Title    string `json:"title"`     // 你加的字段
    Body     string `json:"body"`      // 你加的字段
    ImageURL string `json:"image_url"` // 你加的字段
}
```

生成的 model：

```go
Content *accessor.CampaignContentOption `gorm:"column:content;default:'';serializer:json" json:"content"`
```

读写时 gorm 自动做 JSON 序列化/反序列化，你不用写任何转换代码。

#### 场景 C：props 虚拟字段（数据库无列，纯内存，手动赋值）

`props` 用于声明**数据库表中不存在的字段**（如计算结果、关联聚合等），用 `gen.FieldNew` 生成，**不参与数据库读写**。

以 `campaign.user_count`（参与用户数，运行时算出来）为例：

```yaml
props:
  user_count:
    type: CampaignUserCountOption
```

**1. 生成的 accessor 文件**（带 `Value()`/`Scan()` 空方法，但**用不到**，你只需加字段）：

```go
package accessor

import "database/sql/driver"

type CampaignUserCountOption struct {
    Count int64   // 你加的字段
}

// 这两个空方法用不到（虚拟字段不落库），可留着不管
func (c CampaignUserCountOption) Value() (driver.Value, error) { return c, nil }
func (c *CampaignUserCountOption) Scan(value interface{}) error { return nil }
```

**2. 生成的 model**（注意：**只有 json tag，没有 gorm tag**）：

```go
type Campaign struct {
    ID        int32                              `gorm:"column:id;primaryKey" json:"id"`
    Name      string                             `gorm:"column:name" json:"name"`
    UserCount *accessor.CampaignUserCountOption  `json:"userCount"`
}
```

要点：字段名自动转为大驼峰（`user_count` → `UserCount`），json tag 命名受 `tag_json_camel` 控制。

**3. 业务代码手动赋值**（关键：虚拟字段不落库，你要自己算、自己填）：

```go
// 查活动
var campaign entity.Campaign
db.First(&campaign, 1)

// 手动查参与用户数
var count int64
db.Model(&entity.UserCampaignLog{}).
    Where("campaign_id = ?", campaign.ID).
    Count(&count)

// 手动填进虚拟字段
campaign.UserCount = &accessor.CampaignUserCountOption{Count: count}

fmt.Println(campaign.UserCount.Count) // 参与用户总数
```

#### 三种场景对比总结

- **类型名必须以 `Option` 结尾**，才会触发自定义类型生成
- **场景 A vs B**：有没有 `serializer`。有 `serializer: json` 时 gorm 自动序列化（B）；没有时你必须自己实现 `Value()`/`Scan()`（A）
- **场景 C（props）**：数据库没这列，gorm 不会读写它，所以 `Value()`/`Scan()` 用不到，只需加 struct 字段并在业务代码里手动赋值

### 2.4 relate — 关联关系配置

`relate` 声明当前表与其它表的关联。支持四种关联类型：

| `type` 取值 | 对应 gorm 关系 | 生成字段类型 | 适用场景 |
|-------------|---------------|-------------|---------|
| `has_one` | 一对一（拥有一个） | `*Model`（指针） | 活动有一份配置 |
| `has_many` | 一对多（拥有多个） | `[]Model` | 用户有多条历史 |
| `many_many` | 多对多 | `[]Model` | 用户打多个标签 |
| `belongs_to` | 从属（属于一个） | `*Model`（指针） | 记录属于某个用户 |

关联字段配置项：

| 字段 | 作用 | 适用类型 |
|------|------|---------|
| `table` | 关联的目标表名（必填） | 全部 |
| `type` | 关联类型（必填） | 全部 |
| `name` | 关联字段名，用于同一张表多次关联时区分 | 全部 |
| `foreign_key` | 当前表的外键（关联到对方的字段） | 全部 |
| `references` | 目标表被关联的字段（通常是主键） | 全部 |
| `join_foreign_key` | 中间表中指向当前表的字段 | 仅 `many_many` |
| `join_references` | 中间表中指向目标表的字段 | 仅 `many_many` |
| `many_2_many` | 中间表名 | 仅 `many_many` |

**关联字段名的命名规则**：

- `name` 非空时，用它作为生成的字段名。
  - 含下划线（`include_tags`）→ 自动转大驼峰（`IncludeTags`）
  - 已是驼峰（`ExcludeTags`）→ 原样保留
- `name` 为空时，回退到目标表的 model 结构体名（如关联 `tag` 表 → 字段名 `Tag`）。

#### 各关联类型示例

**has_one（一对一）** —— 活动有一份发送配置：

```yaml
- table: campaign
  relate:
    - table: campaign_config
      foreign_key: campaign_id
      type: has_one
```

生成：`CampaignConfig *CampaignConfig`

**has_many（一对多）** —— 用户有多条参与历史：

```yaml
- table: user
  relate:
    - table: user_campaign_log
      foreign_key: user_id
      type: has_many
```

生成：`UserCampaignLog []UserCampaignLog`

**has_many 关联同一张表多次（用 name 区分）** —— 用户既发送又接收消息：

```yaml
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
```

生成：

```go
SentMessages     []Message `gorm:"foreignKey:sender_id" json:"sent_messages"`
ReceivedMessages []Message `gorm:"foreignKey:receiver_id" json:"received_messages"`
```

> 关键：同一张表被 has_many 关联多次时，必须用 `name` 指定唯一字段名，并用不同的 `foreign_key` 区分（如 `sender_id` / `receiver_id`）。

**belongs_to（从属）** —— 记录属于某个用户和活动：

```yaml
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
```

生成：`User *User`、`Campaign *Campaign`

**many_many（多对多）** —— 用户打标签：

```yaml
- table: user
  relate:
    - table: tag
      many_2_many: user_tag
      foreign_key: id
      join_foreign_key: user_id
      join_references: tag_id
      references: id
      type: many_many
```

生成：`Tag []Tag`

### 2.5 同一张表多次关联（重点）

当一张表需要与**同一张目标表**建立多个关联时（例如邮件活动既「包含某些标签」又「排除某些标签」，都指向 `tag` 表），用 `name` 为每个关联指定唯一字段名，避免字段名冲突：

```yaml
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
```

生成的 model：

```go
type Campaign struct {
    // ...
    IncludeTags []Tag `gorm:"...many2many:campaign_tag_include..." json:"include_tags"`
    ExcludeTags []Tag `gorm:"...many2many:campaign_tag_exclude..." json:"exclude_tags"`
}
```

---

# 生成结果的确定性

同一份 yaml 反复执行，产物应当逐字节一致。为此生成器做了三件事：

- **表按表名字典序生成**：既不依赖 yaml 里的书写顺序，也不依赖 Go map 的随机遍历顺序；
- **`column` / `props` / `tag` 按 key 字典序处理**：字段与 struct tag 的顺序固定；
- **关联目标先完整生成再取快照**：`gen.FieldRelate` 会把目标表“当时”的关联快照进字段，
  所以先递归生成被关联表，再用访问标记兜住双向/环状关联（回边只保留一级关联）。

因此关联字段的嵌套结构只由 yaml 的依赖关系决定，与运行顺序无关。
若发现两次生成结果不同，多半是没升级到包含该修复的版本。

---

# 测试

测试放在 `test/` 目录，按场景拆分到多个文件，每个文件验证一类配置：

| 文件 | 验证内容 |
|------|---------|
| `test/config_test.go` | `tag_json_camel`、`column` 各配置项（type/rename/comment/tag） |
| `test/audit_field_test.go` | 审计字段的两种配置方式（方式 A 字段名 / 方式 B tag） |
| `test/custom_field_test.go` | 自定义字段三种场景（普通自定义 / json / props 虚拟字段） |
| `test/relate_has_one_test.go` | 一对一关联 |
| `test/relate_has_many_test.go` | 一对多关联 |
| `test/relate_belongs_to_test.go` | 从属关联 |
| `test/relate_many_many_test.go` | 多对多关联（含同一张表多次关联） |
| `test/integration_test.go` | 基于 `gen.yaml` + `test.db` 的完整集成验证 |

运行测试：

```bash
go test ./test/
```
