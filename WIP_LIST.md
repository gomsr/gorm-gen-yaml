# WIP List

后续待处理事项，按优先级排列；已完成的收在文末，方便回溯。

---

## 1. accessor 只写不覆盖（P1）

**现象**：改了 yaml 里 `column` 的 `serializer` 或 `type`，重新生成后 `source/gen/accessor/*.go` 内容不变，看起来像"生成没生效"。

**现状代码**（`yaml_generator.go:122` `generateColumnOption`）：

```go
p := y.columnOptionSaveDir + "/" + CamelCaseToUnderscore(column.Type) + ".go"
if _, err := os.Stat(p); os.IsNotExist(err) {
    return os.WriteFile(p, []byte(columnOptionTemplate), 0640)
}
return nil
```

只在文件**不存在**时写。当初是为了让用户能手工调整 `Value/Scan` 而不被生成器覆盖。

**影响**：

- 改 `serializer`（如 `json` → `unixtime`）后必须手动删文件才生效；
- 与 F1~F3 修好的"可复现"形成反差：产物依赖历史状态，新机器和本机结果不同。

**方案**：

- `config` 增加 `overwrite_accessor`（bool，默认 `false` 保持兼容）；
- 开启时**读旧内容比对**，不同才写，避免每次生成都刷新 mtime（否则 IDE / 增量构建会一直抖）；
- 关闭时行为与 v1.2.0 完全一致。

```yaml
config:
  overwrite_accessor: true
```

**验收**：

- 开启后修改 `serializer` 重跑，accessor 文件内容更新；
- 内容无变化时重跑，mtime 不变；
- 默认关闭时 `git status` 与 v1.2.0 行为一致。

**风险**：会覆盖用户手改过的 accessor，因此默认关闭，并在 README 写清"开启后该目录归生成器所有"。

---

## 已完成

- ~~RelateTableMap 清理~~：已按方案 B 处理（保留字段 + `// Deprecated` 标注），详见 FEATURE_LIST F5。下个大版本可考虑直接删除。

---

## 备注：外部依赖风险（不改本仓库）

| 风险 | 位置 | 当前判断 |
|---|---|---|
| 包级全局 `importList` 在多 goroutine 中并发 `Add` | `gorm.io/gen@v0.3.26/import.go` | 理论数据竞争；实测 5 次运行 `dal/gen.go` 一致，暂无可见症状 |
| pgsql `ColumnTypes` 查询无 `ORDER BY`（MySQL 驱动有 `ORDINAL_POSITION`） | `gorm.io/driver/postgres/migrator.go` | 实测 `model/*.gen.go` 稳定；大表/分区表存在理论顺序风险，升级驱动时复核 |

两条都不在本仓库范围内，暂不处理，仅登记。
