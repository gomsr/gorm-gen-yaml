-- 邮件营销系统示例数据库结构
-- 覆盖 gorm 常见关联类型：一对一、一对多、多对多、同一张表多次关联
-- 业务表含审计字段，中间表不含
--
-- 审计字段两种写法（见 gen.yaml 配置说明）：
--   方式 A：字段名 created_at / updated_at / deleted_at，gorm 靠字段名自动识别
--   方式 B：字段名 create_time / update_time / delete_time，靠 autoCreateTime/autoUpdateTime tag 指定
--
-- 使用 sqlite3 生成 test.db：
--   sqlite3 test.db < schema.sql

-- ============ 业务表 ============

-- 用户表（方式 B：用 create_time/update_time/delete_time + autoCreateTime/autoUpdateTime tag）
CREATE TABLE user (
    id          INTEGER PRIMARY KEY,
    name        TEXT,
    email       TEXT,
    create_time DATETIME,
    update_time DATETIME,
    delete_time DATETIME
);

-- 标签表（用于圈选/排除目标用户，方式 A：字段名 created_at 自动识别）
CREATE TABLE tag (
    id         INTEGER PRIMARY KEY,
    name       TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);

-- 邮件活动表（方式 A）
CREATE TABLE campaign (
    id         INTEGER PRIMARY KEY,
    name       TEXT,
    status     TEXT,
    content    TEXT,   -- 存 JSON 字符串（情况 B：json 类型字段）
    tags       TEXT,   -- 存逗号分隔字符串（情况 A：普通自定义类型）
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);

-- 用户活动参与历史表（一对多 / 从属的载体，方式 A）
CREATE TABLE user_campaign_log (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER,
    campaign_id INTEGER,
    action      TEXT,
    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME
);

-- 活动发送配置表（一对一，方式 A）
CREATE TABLE campaign_config (
    id          INTEGER PRIMARY KEY,
    campaign_id INTEGER,
    subject     TEXT,
    send_time   TEXT,
    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME
);

-- ============ 中间表（关联表，不含审计字段）============

-- 用户-标签（多对多：用户身上的标签）
CREATE TABLE user_tag (
    id      INTEGER PRIMARY KEY,
    user_id INTEGER,
    tag_id  INTEGER
);

-- 活动-包含标签（多对多，同一张表多次关联之一）
CREATE TABLE campaign_tag_include (
    id          INTEGER PRIMARY KEY,
    campaign_id INTEGER,
    tag_id      INTEGER
);

-- 活动-排除标签（多对多，同一张表多次关联之二）
CREATE TABLE campaign_tag_exclude (
    id          INTEGER PRIMARY KEY,
    campaign_id INTEGER,
    tag_id      INTEGER
);

-- ============ 示例数据 ============

INSERT INTO user (id, name, email) VALUES
    (1, '张三', 'zhangsan@example.com'),
    (2, '李四', 'lisi@example.com'),
    (3, '王五', 'wangwu@example.com');

INSERT INTO tag (id, name) VALUES
    (1, 'VIP'),
    (2, '新用户'),
    (3, '活跃用户');

INSERT INTO user_tag (id, user_id, tag_id) VALUES
    (1, 1, 1),
    (2, 1, 3),
    (3, 2, 2),
    (4, 3, 3);

INSERT INTO campaign (id, name, status, content, tags) VALUES
    (1, '618 大促活动', 'draft', '{"title":"618大促","body":"全场五折"}', 'vip,new'),
    (2, '会员专享活动', 'sending', '{"title":"会员专享","body":"限时优惠"}', 'vip');

INSERT INTO campaign_tag_include (id, campaign_id, tag_id) VALUES
    (1, 1, 1),
    (2, 1, 3);

INSERT INTO campaign_tag_exclude (id, campaign_id, tag_id) VALUES
    (1, 1, 2);

INSERT INTO user_campaign_log (id, user_id, campaign_id, action) VALUES
    (1, 1, 1, 'open'),
    (2, 1, 1, 'click'),
    (3, 3, 1, 'open');

INSERT INTO campaign_config (id, campaign_id, subject, send_time) VALUES
    (1, 1, '618 大促，全场五折', '2024-06-18 10:00:00');
