# API 接口文档

## 认证说明

- 需要登录：请求需携带有效的 `session_token` Cookie
- 需要站长：需要登录且用户角色为 `admin`

---

## 健康检查

| 方法 | 路径 | 简介 | 需要登录 | 需要站长 |
|------|------|------|:--------:|:--------:|
| GET | `/health` | 健康检查 | ❌ | ❌ |

---

## 认证 (Auth)

| 方法 | 路径 | 简介 | 需要登录 | 需要站长 |
|------|------|------|:--------:|:--------:|
| GET | `/api/v1/auth/signin/:provider` | 跳转到 OAuth 提供商进行登录 | ❌ | ❌ |
| GET | `/api/v1/auth/callback/:provider` | OAuth 回调处理 | ❌ | ❌ |
| GET | `/api/v1/auth/session` | 获取当前用户信息 | 可选 | ❌ |
| POST | `/api/v1/auth/signout` | 登出 | ❌ | ❌ |
| PUT | `/api/v1/auth/profile` | 更新用户信息 | ✅ | ❌ |

---

## Schema 管理

| 方法 | 路径 | 简介 | 需要登录 | 需要站长 |
|------|------|------|:--------:|:--------:|
| POST | `/api/v1/schemas` | 创建 Schema | ✅ | ✅ |
| GET | `/api/v1/schemas` | 获取 Schema 列表 | ✅ | ✅ |
| GET | `/api/v1/schemas/:key` | 获取指定 Schema | ✅ | ✅ |
| DELETE | `/api/v1/schemas/:key` | 删除 Schema | ✅ | ✅ |

---

## 内容 (Entry)

| 方法 | 路径 | 简介 | 需要登录 | 需要站长 |
|------|------|------|:--------:|:--------:|
| GET | `/api/v1/entries` | 获取内容列表（支持搜索、分页） | 可选 | ❌ |
| GET | `/api/v1/entries/:id` | 获取指定内容 | 可选 | ❌ |
| POST | `/api/v1/entries` | 创建内容 | ✅ | ❌ |
| PUT | `/api/v1/entries/:id` | 更新内容（作者或站长） | ✅ | ❌ |
| DELETE | `/api/v1/entries/:id` | 删除内容（作者或站长） | ✅ | ❌ |

---

## 分类法 (Taxonomy)

| 方法 | 路径 | 简介 | 需要登录 | 需要站长 |
|------|------|------|:--------:|:--------:|
| GET | `/api/v1/taxonomies` | 获取分类法列表 | ❌ | ❌ |
| GET | `/api/v1/taxonomies/:key` | 获取指定分类法 | ❌ | ❌ |
| POST | `/api/v1/taxonomies` | 创建分类法 | ✅ | ✅ |
| PUT | `/api/v1/taxonomies/:key` | 更新分类法 | ✅ | ✅ |
| DELETE | `/api/v1/taxonomies/:key` | 删除分类法 | ✅ | ✅ |

---

## 分类项 (Term)

| 方法 | 路径 | 简介 | 需要登录 | 需要站长 |
|------|------|------|:--------:|:--------:|
| GET | `/api/v1/terms/taxonomy/:key` | 获取指定分类法下的所有分类项 | ❌ | ❌ |
| GET | `/api/v1/terms/:id` | 获取指定分类项 | ❌ | ❌ |
| POST | `/api/v1/terms` | 创建分类项 | ✅ | ✅ |
| PUT | `/api/v1/terms/:id` | 更新分类项 | ✅ | ✅ |
| DELETE | `/api/v1/terms/:id` | 删除分类项 | ✅ | ✅ |

---

## 评论 (Comment)

| 方法 | 路径 | 简介 | 需要登录 | 需要站长 |
|------|------|------|:--------:|:--------:|
| GET | `/api/v1/comments/entry/:entry_id` | 获取指定内容的评论列表 | ❌ | ❌ |
| POST | `/api/v1/comments` | 创建评论 | ✅ | ❌ |
| PUT | `/api/v1/comments/:id` | 更新评论（仅作者） | ✅ | ❌ |
| DELETE | `/api/v1/comments/:id` | 删除评论（作者或站长） | ✅ | ❌ |

---

## 表情反应 (Reaction)

| 方法 | 路径 | 简介 | 需要登录 | 需要站长 |
|------|------|------|:--------:|:--------:|
| POST | `/api/v1/reactions` | 添加表情反应 | ✅ | ❌ |
| DELETE | `/api/v1/reactions` | 移除表情反应 | ✅ | ❌ |
| GET | `/api/v1/reactions` | 获取目标的反应统计 | 可选 | ❌ |
| POST | `/api/v1/reactions/batch` | 批量获取多个目标的反应统计 | 可选 | ❌ |

---

## 权限说明

- **可选**：未登录用户可访问，登录用户可获得额外信息（如用户自己的反应状态）
- **作者或站长**：内容作者可操作自己的内容，站长可操作所有内容
- **仅作者**：只有内容作者可以操作
