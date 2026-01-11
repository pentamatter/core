# Requirements Document

## Introduction

本功能为 Entry 和 Comment 添加 emoji reaction 系统，类似 GitHub 的 reactions 功能。用户可以对内容发表 emoji 反应，系统使用 Redis 存储用户的 reaction 记录，MongoDB 存储聚合后的 reaction 统计数据。

## Glossary

- **Reaction_System**: 处理 emoji reaction 的核心系统
- **Target**: reaction 的目标对象，可以是 Entry 或 Comment
- **Target_Type**: 目标类型枚举，值为 "entry" 或 "comment"
- **Reaction_Type**: 任意有效的 emoji 字符
- **Reaction_Summary**: 存储在 MongoDB 中的 reaction 聚合统计
- **User_Reaction**: 存储在 Redis 中的用户 reaction 记录
- **Redis_Repository**: 负责 Redis 数据操作的仓库层

## Requirements

### Requirement 1: 添加 Reaction

**User Story:** As a 登录用户, I want to 对 Entry 或 Comment 添加 emoji reaction, so that 我可以表达对内容的态度。

#### Acceptance Criteria

1. WHEN 用户对目标添加 reaction, THE Reaction_System SHALL 在 Redis 中记录该用户对该目标的 reaction
2. WHEN 用户对目标添加 reaction, THE Reaction_System SHALL 在 MongoDB 中增加该目标对应 Reaction_Type 的计数
3. WHEN 用户对同一目标重复添加相同 reaction, THE Reaction_System SHALL 拒绝操作并返回错误提示
4. WHEN 用户对同一目标添加不同 reaction, THE Reaction_System SHALL 允许操作并分别记录
5. IF 目标不存在, THEN THE Reaction_System SHALL 返回 404 错误
6. IF 用户未登录, THEN THE Reaction_System SHALL 返回 401 错误
7. IF Reaction_Type 不是有效的 emoji 字符, THEN THE Reaction_System SHALL 返回 400 错误

### Requirement 2: 移除 Reaction

**User Story:** As a 登录用户, I want to 移除我之前添加的 reaction, so that 我可以撤回我的态度表达。

#### Acceptance Criteria

1. WHEN 用户移除已存在的 reaction, THE Reaction_System SHALL 从 Redis 中删除该用户对该目标的 reaction 记录
2. WHEN 用户移除已存在的 reaction, THE Reaction_System SHALL 在 MongoDB 中减少该目标对应 Reaction_Type 的计数
3. WHEN 计数减少后为零, THE Reaction_System SHALL 从 Reaction_Summary 中移除该 Reaction_Type
4. IF 用户尝试移除不存在的 reaction, THEN THE Reaction_System SHALL 返回 404 错误
5. IF 目标不存在, THEN THE Reaction_System SHALL 返回 404 错误

### Requirement 3: 查询 Reaction 统计

**User Story:** As a 用户, I want to 查看 Entry 或 Comment 的 reaction 统计, so that 我可以了解其他人对内容的态度。

#### Acceptance Criteria

1. WHEN 查询目标的 reaction 统计, THE Reaction_System SHALL 返回该目标所有 Reaction_Type 及其对应计数
2. WHEN 已登录用户查询 reaction 统计, THE Reaction_System SHALL 同时返回当前用户已添加的 reaction 列表
3. WHEN 目标没有任何 reaction, THE Reaction_System SHALL 返回空的统计结果
4. THE Reaction_System SHALL 从 MongoDB 读取 Reaction_Summary 数据
5. THE Reaction_System SHALL 从 Redis 读取当前用户的 User_Reaction 数据

### Requirement 4: 批量查询 Reaction 统计

**User Story:** As a 用户, I want to 批量查询多个目标的 reaction 统计, so that 在列表页面可以高效获取所有内容的 reaction 信息。

#### Acceptance Criteria

1. WHEN 批量查询多个目标的 reaction 统计, THE Reaction_System SHALL 返回每个目标的 Reaction_Summary
2. WHEN 已登录用户批量查询, THE Reaction_System SHALL 同时返回当前用户对每个目标已添加的 reaction 列表
3. IF 批量查询数量超过 100, THEN THE Reaction_System SHALL 返回 400 错误
4. THE Reaction_System SHALL 支持混合查询 Entry 和 Comment 类型的目标

### Requirement 5: Reaction 数据模型

**User Story:** As a 开发者, I want to 有清晰的数据模型定义, so that 系统可以正确存储和查询 reaction 数据。

#### Acceptance Criteria

1. THE Reaction_Summary SHALL 包含 target_id、target_type 和 reactions 字段
2. THE reactions 字段 SHALL 为 map 结构，key 为 emoji 字符串，value 为计数
3. THE User_Reaction 在 Redis 中 SHALL 使用 Set 结构存储，key 格式为 `user_reactions:{user_id}`，value 为 `{target_type}:{target_id}:{emoji}` 格式的记录
4. THE Reaction_System SHALL 支持任意有效的 emoji 字符作为 Reaction_Type
5. THE Reaction_System SHALL 验证输入的 Reaction_Type 是否为有效的 emoji 字符

### Requirement 6: Redis 集成

**User Story:** As a 开发者, I want to 集成 Redis 作为 User_Reaction 的存储, so that 可以高效查询用户的 reaction 记录。

#### Acceptance Criteria

1. THE Redis_Repository SHALL 提供连接池管理功能
2. THE Redis_Repository SHALL 支持从环境变量读取 Redis 连接配置
3. WHEN Redis 连接失败, THE Reaction_System SHALL 记录错误日志并返回 500 错误
4. THE Redis_Repository SHALL 为 User_Reaction 设置合理的过期时间或使用持久化存储

### Requirement 7: 数据一致性

**User Story:** As a 开发者, I want to 确保 Redis 和 MongoDB 数据一致, so that 用户看到的 reaction 信息是准确的。

#### Acceptance Criteria

1. WHEN 添加 reaction 时, THE Reaction_System SHALL 先写入 Redis，再更新 MongoDB
2. IF MongoDB 更新失败, THEN THE Reaction_System SHALL 回滚 Redis 中的记录
3. WHEN 移除 reaction 时, THE Reaction_System SHALL 先更新 MongoDB，再删除 Redis 记录
4. IF Redis 删除失败, THEN THE Reaction_System SHALL 记录错误日志但不影响响应

### Requirement 8: API 端点设计

**User Story:** As a 前端开发者, I want to 有清晰的 API 端点, so that 我可以正确调用 reaction 功能。

#### Acceptance Criteria

1. THE Reaction_System SHALL 提供 `POST /api/reactions` 端点用于添加 reaction
2. THE Reaction_System SHALL 提供 `DELETE /api/reactions` 端点用于移除 reaction
3. THE Reaction_System SHALL 提供 `GET /api/reactions` 端点用于查询单个目标的 reaction 统计
4. THE Reaction_System SHALL 提供 `POST /api/reactions/batch` 端点用于批量查询 reaction 统计
5. THE 添加和移除 reaction 的请求体 SHALL 包含 target_id、target_type 和 reaction_type 字段



