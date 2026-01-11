package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- Enums ---
type FieldType string

const (
	TypeString   FieldType = "string"
	TypeNumber   FieldType = "number"
	TypeBool     FieldType = "bool"
	TypeDate     FieldType = "date"
	TypeObject   FieldType = "object"
	TypeArray    FieldType = "array"
	TypeTaxonomy FieldType = "taxonomy"
)

type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// --- 1. Schema (Immutable, Versioned) ---
type FieldSchema struct {
	Key      string    `bson:"key" json:"key"`
	Label    string    `bson:"label" json:"label"`
	Type     FieldType `bson:"type" json:"type"`
	Required bool      `bson:"required" json:"required"`
	Default  any       `bson:"default,omitempty" json:"default,omitempty"`

	// Complex Types
	Children      []FieldSchema `bson:"children,omitempty" json:"children,omitempty"`
	ItemType      *FieldSchema  `bson:"item_type,omitempty" json:"item_type,omitempty"`
	TaxonomyKey   string        `bson:"taxonomy_key,omitempty" json:"taxonomy_key,omitempty"`
	AllowMultiple bool          `bson:"allow_multiple,omitempty" json:"allow_multiple,omitempty"`
}

type Schema struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Key       string             `bson:"key" json:"key"`
	Version   int                `bson:"version" json:"version"`
	Name      string             `bson:"name" json:"name"`
	Fields    []FieldSchema      `bson:"fields" json:"fields"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// --- 2. Entry (Dynamic Content) ---
type BaseMeta struct {
	Title     string    `bson:"title" json:"title"`
	Slug      string    `bson:"slug" json:"slug"`
	Draft     bool      `bson:"draft" json:"draft"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

type Entry struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SchemaID      primitive.ObjectID `bson:"schema_id" json:"schema_id"`
	SchemaKey     string             `bson:"schema_key" json:"schema_key"`
	SchemaVersion int                `bson:"schema_version" json:"schema_version"`
	AuthorID      string             `bson:"author_id" json:"author_id"`

	Base       BaseMeta       `bson:"base" json:"base"`
	Body       string         `bson:"body" json:"body"`
	Attributes map[string]any `bson:"attributes" json:"attributes"`
}

// --- 3. Taxonomy & Terms ---
type Taxonomy struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Key            string             `bson:"key" json:"key"`
	Name           string             `bson:"name" json:"name"`
	IsHierarchical bool               `bson:"is_hierarchical" json:"is_hierarchical"`
}

type Term struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TaxonomyKey string             `bson:"taxonomy_key" json:"taxonomy_key"`
	Name        string             `bson:"name" json:"name"`
	Slug        string             `bson:"slug" json:"slug"`
	Color       string             `bson:"color" json:"color"`
	ParentID    primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id"`
}

// --- 4. Comments (Two-Level Flat) ---
type Comment struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	EntryID  primitive.ObjectID `bson:"entry_id" json:"entry_id"`
	AuthorID string             `bson:"author_id" json:"author_id"`

	RootID     primitive.ObjectID `bson:"root_id,omitempty" json:"root_id"`
	ParentID   primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id"`
	ReplyToUID string             `bson:"reply_to_uid,omitempty" json:"reply_to_uid"`

	Content   string    `bson:"content" json:"content"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// CommentWithAuthor 包含作者信息的评论
type CommentWithAuthor struct {
	Comment `bson:",inline"`
	Author  *UserPublic `bson:"author" json:"author"`
}

// --- 5. User (OAuth2) ---
type SocialBind struct {
	Provider       string `bson:"provider" json:"provider"`
	ProviderUserID string `bson:"provider_user_id" json:"-"` // 隐藏敏感信息
	Name           string `bson:"name" json:"name"`
	Email          string `bson:"email" json:"-"` // 隐藏敏感信息
	Avatar         string `bson:"avatar" json:"avatar"`
}

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Role      string             `bson:"role" json:"role"`
	Nickname  string             `bson:"nickname" json:"nickname"`
	Avatar    string             `bson:"avatar" json:"avatar"`
	Email     string             `bson:"email" json:"email,omitempty"` // 仅管理员或本人可见
	Socials   []SocialBind       `bson:"socials" json:"socials"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// UserPublic 用于公开展示的用户信息
type UserPublic struct {
	ID       primitive.ObjectID `json:"id"`
	Nickname string             `json:"nickname"`
	Avatar   string             `json:"avatar"`
}

// --- 6. Session ---
type Session struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Token     string             `bson:"token" json:"token"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Role      string             `bson:"role" json:"role"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
}

// --- 7. OAuth State (for CSRF protection) ---
type OAuthState struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	State     string             `bson:"state" json:"state"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
}

// --- Search Document for Meilisearch ---
type SearchDocument struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	SchemaKey string `json:"schema_key"`
	AllText   string `json:"all_text"`
}
