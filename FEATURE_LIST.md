# Feature List

基线：v1.2.0 → v1.2.1（候选）
背景：同一份 `relation.yaml` + `generate.go`，多次运行 `source/gen/dal/*.gen.go` 每次有 6~9 个文件变化，差异全部是嵌套关联 `field.NewRelation("X.Y", ...)` 的有无。

根因两条：一是遍历顺序来自 Go map（随机）；二是 `gen.FieldRelate` 会把目标表「当时」的 `Relations()` 快照进字段，而目标表常常只是一个不含关联的「裸模型」。

---

## F1 生成顺序可复现

**问题**：`Generate()` 直接 `range y.yaml.TableMap`，`getTableColumnOpt()` 直接 `range table.Column/Props/Tag`。Go 的 map 遍历顺序每个进程都不同，字段与 tag 的落地顺序随之漂移。

**改动**（`yaml_generator.go`）：

- `Generate()`（:394）：`TableMap` 改为 `sortedKeys()` 后按表名字典序遍历；
- `getTableColumnOpt()`：`column`（:214）、`tagType`（:247）、`tagName`（:251、:260）、`props`（:280）全部改为有序遍历；
- 新增 `sortedKeys()`（:378）：泛型小工具，取 key 排序后返回。

**行为**：输出顺序只由 yaml 内容决定，与运行次数、机器、进程无关。yaml 里调整表/字段的书写顺序不再影响产物。

**验证**：同一份 yaml 连跑 5 次 `diff -r` 为 0；rpc-sms（39 张表、pgsql）连跑 3 次 0 差异。

---

## F2 关联关系完整且稳定

**问题**：旧逻辑用 `RelateTableMap`（"被任何表引用过"的表集合）判断是否递归：

```go
if !isInRelateTable {          // 没被引用过 → 递归
    y.generateFromTable(...)
} else {                       // 被引用过 → 当成疑似环，裸模型占位
    y.gen.GenerateModel(relate.Table, opt...)
}
```

判据把「被引用过」当成了「在当前调用栈上」，几乎所有关联目标都落进 else 分支，父表拿到的快照就是"零关联"。唯一例外是目标表恰好已被顶层遍历完整生成过 —— 于是同一份 yaml 时好时坏。

**改动**：

- `generateFromTable()`（:318）：目标表若在 yaml 顶层声明，**先递归完整生成**（带它自己的 relate/column 定制），再计算 `FieldRelate`；
- `inProgress`（:27、:309-313）：精确 DFS 栈标记，只拦截真正的回边，A→B→C 这类链照常递归；
- 新增 `ensureBaseModel()`（:356）：环上的回边、只出现在 `relate` 里的表，兜底生成基础模型，保证有元数据可取。

**行为变化**（需写进 release note）：

- 双向关联的回边固定保留**一级**关联，不再出现"这次有嵌套、下次没有"；
- 升级后首次生成 diff 会明显变大（补回此前随机丢失的多级关联），属预期。

**验证**：`sms_draw.gen.go` 中 `NewRelation("SmsDraw…)` 数量，修复前 2/3/4 乱跳，修复后稳定为 4。

---

## F3 关联解析 fail-fast

**问题**：旧代码 `y.gen.Data[y.generatedTable[table.Table].ModelName]` 一路解引用，一旦目标表没生成就是无上下文的 nil pointer panic，排查困难。

**改动**：`getTableRelateOpt()`（:146 起）对 `generatedTable`、`gen.Data` 双重判空，报错带上表名与模型名：

```go
panic(fmt.Errorf("relation table <%s> of <%s> is not generated yet", ...))
```

**验证**：yaml 里引用一张不存在的表，报错可直接定位到表。

---

## F4 回归测试与文档

- `test/determinism_test.go`：构造 `author ⇄ article ⇄ comment` 双向关联，连续生成 5 次、逐文件字节比对；选双向关联是因为只有它能稳定暴露 F1+F2 的组合问题。
- README 增加「生成结果的确定性」，写明三条保证：表名字典序、字段与 tag 字典序、关联先完整生成再取快照。

**注意**：该用例依赖 sqlite，需要 cgo。本机 `CGO_ENABLED=0` 且无 gcc，未能本地执行，请在 macOS / Linux / CI 跑一次 `go test -run TestGenerateDeterministic -v ./test/`。

---

## F5 `RelateTableMap` 标记废弃

**问题**：该字段承载旧版"粗粒度环检测"（把"被引用过"当成"成环"），正是 F2 抖动成因之一。留着不标注，后来者容易照着旧逻辑加判断，把顺序依赖重新引回来。

**改动**（保留字段，不动结构）：

- `DbTable.RelateTableMap`（`:47` 起）加 `// Deprecated: 环检测请使用 inProgress`，并说明"入度 > 0 无法区分被引用与在调用栈上"；
- `loadFromFile()` 的填充处（`:107`）同步标注"仅为兼容保留，生成流程已不再读取"。

**为什么选保留而非删除**：字段是导出的，外部可能有代码读取；先标废弃、下个大版本再删，升级成本更低。

**验证**：`go build ./...` 通过；IDE/gopls 对该字段的使用会给出废弃提示。

---

## 兼容性

- 公共 API 未变：`NewYamlGenerator` / `UseGormGenerator` / `Generate` 签名保持一致；
- yaml 语义未变，`column` / `props` / `relate` 的用法不变；
- 唯一可见变化是**产出的代码内容**（顺序固定 + 关联补齐），业务侧 `Preload` 写法不受影响。
