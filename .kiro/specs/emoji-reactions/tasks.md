# Implementation Plan: Emoji Reactions System

## Overview

实现类似 GitHub 的 emoji reaction 系统，支持对 Entry 和 Comment 添加/移除 emoji 反应。采用 Redis + MongoDB 双存储架构：Redis 存储用户维度的 reaction 记录，MongoDB 存储聚合统计。

## Tasks

- [x] 1. 配置和数据模型
  - [x] 1.1 扩展配置支持 Redis 连接参数
    - 在 `internal/config/config.go` 中添加 Redis 配置字段（RedisAddr, RedisPassword, RedisDB）
    - 从环境变量读取配置
    - _Requirements: 6.2_

  - [x] 1.2 创建 Reaction 数据模型
    - 在 `internal/model/model.go` 中添加 TargetType、ReactionSummary、ReactionResponse 类型
    - TargetType 枚举值为 "entry" 和 "comment"
    - ReactionSummary 包含 target_id、target_type、reactions map、updated_at
    - _Requirements: 5.1, 5.2_

- [x] 2. Redis Repository 实现
  - [x] 2.1 创建 Redis Repository 基础结构
    - 创建 `internal/repository/redis.go`
    - 实现 NewRedisRepo、Close、Ping 方法
    - 使用 go-redis 客户端库
    - _Requirements: 6.1, 6.2_

  - [x] 2.2 实现 User Reaction 操作方法
    - AddUserReaction: 添加用户 reaction 到 Set
    - RemoveUserReaction: 从 Set 移除用户 reaction
    - HasUserReaction: 检查用户是否已添加某 reaction
    - GetUserReactionsForTarget: 获取用户对单个目标的所有 reactions
    - GetUserReactionsForTargets: 批量获取用户对多个目标的 reactions
    - Key 格式: `user_reactions:{user_id}`，Value 格式: `{target_type}:{target_id}:{emoji}`
    - _Requirements: 5.3, 6.4_

- [x] 3. MongoDB Repository 扩展
  - [x] 3.1 添加 reaction_summaries collection 和索引
    - 在 MongoRepo 中添加 reactionSummaries collection
    - 创建 `{ target_type: 1, target_id: 1 }` 唯一索引
    - _Requirements: 5.1_

  - [x] 3.2 实现 Reaction Summary 操作方法
    - GetReactionSummary: 获取单个目标的 reaction 统计
    - GetReactionSummaries: 批量获取多个目标的 reaction 统计
    - IncrementReaction: 增加某 emoji 的计数（使用 $inc）
    - DecrementReaction: 减少某 emoji 的计数，计数为 0 时移除该 key
    - _Requirements: 2.3, 3.4_

- [x] 4. Emoji 验证器实现
  - [x] 4.1 创建 Emoji Validator
    - 创建 `internal/service/emoji.go`
    - 实现 IsValidEmoji 方法，验证输入是否为有效 emoji 字符
    - 支持单个 emoji 和组合 emoji（如 👨‍👩‍👧‍👦）
    - _Requirements: 5.4, 5.5_

  - [ ]* 4.2 编写 Emoji Validator 单元测试
    - 测试常见 emoji（👍、❤️、😄 等）
    - 测试拒绝普通文本和空字符串
    - 测试组合 emoji
    - _Requirements: 5.4, 5.5_

- [x] 5. Reaction Service 实现
  - [x] 5.1 创建 Reaction Service 基础结构
    - 创建 `internal/service/reaction.go`
    - 注入 MongoRepo、RedisRepo、EmojiValidator 依赖
    - _Requirements: 1.1, 1.2_

  - [x] 5.2 实现 AddReaction 方法
    - 验证 emoji 有效性
    - 验证目标存在（Entry 或 Comment）
    - 检查是否已存在相同 reaction（返回 409 错误）
    - 先写入 Redis，再更新 MongoDB
    - MongoDB 更新失败时回滚 Redis
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.7, 7.1, 7.2_

  - [x] 5.3 实现 RemoveReaction 方法
    - 验证目标存在
    - 检查 reaction 是否存在（返回 404 错误）
    - 先更新 MongoDB，再删除 Redis
    - Redis 删除失败时记录日志但不影响响应
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 7.3, 7.4_

  - [x] 5.4 实现 GetReactions 方法
    - 从 MongoDB 获取 ReactionSummary
    - 如果用户已登录，从 Redis 获取用户的 reactions
    - 返回 ReactionResponse
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [x] 5.5 实现 GetReactionsBatch 方法
    - 验证批量数量不超过 100
    - 从 MongoDB 批量获取 ReactionSummaries
    - 如果用户已登录，从 Redis 批量获取用户的 reactions
    - 支持混合查询 Entry 和 Comment
    - _Requirements: 4.1, 4.2, 4.3, 4.4_

- [x] 6. Reaction Handler 实现
  - [x] 6.1 创建 Reaction Handler 基础结构
    - 创建 `internal/handler/reaction.go`
    - 定义请求/响应结构体
    - _Requirements: 8.5_

  - [x] 6.2 实现 Add handler (POST /api/v1/reactions)
    - 解析请求体（target_id, target_type, emoji）
    - 调用 ReactionService.AddReaction
    - 返回更新后的 ReactionResponse
    - _Requirements: 8.1, 8.5_

  - [x] 6.3 实现 Remove handler (DELETE /api/v1/reactions)
    - 解析请求体（target_id, target_type, emoji）
    - 调用 ReactionService.RemoveReaction
    - 返回更新后的 ReactionResponse
    - _Requirements: 8.2, 8.5_

  - [x] 6.4 实现 Get handler (GET /api/v1/reactions)
    - 解析查询参数（target_id, target_type）
    - 调用 ReactionService.GetReactions
    - _Requirements: 8.3_

  - [x] 6.5 实现 GetBatch handler (POST /api/v1/reactions/batch)
    - 解析请求体（targets 数组）
    - 调用 ReactionService.GetReactionsBatch
    - _Requirements: 8.4_

- [x] 7. 路由集成和初始化
  - [x] 7.1 初始化 Redis 连接
    - 在 `cmd/server/main.go` 中初始化 RedisRepo
    - 处理连接失败情况
    - _Requirements: 6.3_

  - [x] 7.2 注册 Reaction 路由
    - POST /api/v1/reactions (需要认证)
    - DELETE /api/v1/reactions (需要认证)
    - GET /api/v1/reactions (可选认证)
    - POST /api/v1/reactions/batch (可选认证)
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 1.6_

- [-] 8. Checkpoint - 确保基础功能完成
  - 确保所有代码编译通过
  - 确保 Redis 和 MongoDB 连接正常
  - 如有问题请询问用户

- [ ]* 9. Property-Based Tests
  - [ ]* 9.1 编写 Property 1: Add Reaction Completeness 测试
    - **Property 1: Add Reaction Completeness**
    - *For any* valid user, target, and emoji, adding a reaction should result in Redis record existing and MongoDB count increasing
    - **Validates: Requirements 1.1, 1.2**

  - [ ]* 9.2 编写 Property 3: Add Reaction Idempotence 测试
    - **Property 3: Add Reaction Idempotence**
    - *For any* user, target, and emoji, adding the same reaction twice should fail on second attempt
    - **Validates: Requirements 1.3**

  - [ ]* 9.3 编写 Property 5: Emoji Validation 测试
    - **Property 5: Emoji Validation**
    - *For any* input string, valid emojis should be accepted, non-emojis should be rejected
    - **Validates: Requirements 1.7, 5.4**

  - [ ]* 9.4 编写 Property 11: Add-Remove Round Trip 测试
    - **Property 11: Add-Remove Round Trip**
    - *For any* user, target, and emoji, adding then removing should return to original state
    - **Validates: Requirements 1.1, 1.2, 2.1, 2.2**

- [ ] 10. Final Checkpoint
  - 确保所有测试通过
  - 确保 API 端点正常工作
  - 如有问题请询问用户

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- 使用 gopter 作为 Go 的属性测试库
- Redis key 格式: `user_reactions:{user_id}`，value 格式: `{target_type}:{target_id}:{emoji}`
- 数据一致性策略：添加时先 Redis 后 MongoDB（失败回滚），移除时先 MongoDB 后 Redis（Redis 失败仅记录日志）
