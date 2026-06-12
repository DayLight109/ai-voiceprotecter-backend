// Package domain 定义业务核心实体（与 DB schema 对齐）。
//
// 各业务表对应一个 struct，统一时间戳类型与可空字段。
// repo 层会把 sqlc 生成的类型映射到这里。
package domain

import "time"

// Role 五种角色
type Role string

const (
	RoleFamily       Role = "family"
	RoleBiz          Role = "biz"
	RoleFamilyAdmin  Role = "family_admin"
	RoleAdmin        Role = "admin"
	RoleSysadmin     Role = "sysadmin"
)

// TenantKind 租户类型
type TenantKind string

const (
	TenantFamily     TenantKind = "family"
	TenantEnterprise TenantKind = "enterprise"
	TenantGlobal     TenantKind = "global"
)

type Tenant struct {
	ID        string     `json:"id"`
	Kind      TenantKind `json:"kind"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"createdAt"`
}

type User struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenantId"`
	Name         string     `json:"name"`
	Phone        string     `json:"phone,omitempty"`
	Email        string     `json:"email,omitempty"`
	Role         Role       `json:"role"`
	Status       string     `json:"status"`     // active | review | suspended
	Dept         string     `json:"dept,omitempty"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type IdentityCredential struct {
	ID         string     `json:"id"`
	UserID     string     `json:"userId"`
	Kind       string     `json:"kind"`     // phone | id_card | passport | military | hk_mo
	Verified   bool       `json:"verified"`
	VerifiedAt *time.Time `json:"verifiedAt,omitempty"`
	Masked     string     `json:"masked,omitempty"`    // 脱敏展示值（"138 ···· 4921"）
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
	Photos     []CredentialPhoto `json:"photos,omitempty"`
}

// CredentialPhoto 证件照片视图（identity 页直接用 dataUrl 渲染 <img>，
// 因为带 Authorization 的 <img src> 无法实现）。
type CredentialPhoto struct {
	Slot      string    `json:"slot"` // face | emblem | main
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Mime      string    `json:"mime"`
	DataURL   string    `json:"dataUrl,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type BlacklistEntry struct {
	ID         string    `json:"id"`
	TenantID   *string   `json:"tenantId,omitempty"`     // NULL = 全局
	Number     string    `json:"number"`
	Reason     string    `json:"reason"`
	Category   string    `json:"category"`
	Risk       int       `json:"risk"`
	Source     string    `json:"source"`
	Dispatched bool      `json:"dispatched"` // false = 举报通过自动入库、待管理员下发
	IsGlobal   bool      `json:"isGlobal"`   // tenant_id IS NULL 的派生字段
	CreatedAt  time.Time `json:"createdAt"`
}

type WhitelistEntry struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	Number    string    `json:"number"`
	Name      string    `json:"name"`
	Relation  string    `json:"relation"`
	CreatedAt time.Time `json:"createdAt"`
}

type KnowledgeArticle struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Summary   string    `json:"summary"`
	Body      string    `json:"body"`
	Views     int64     `json:"views"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ScamRule struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Keyword  string `json:"keyword"`
	Weight   int    `json:"weight"`
	Enabled  bool   `json:"enabled"`
}

type RiskLevelState struct {
	TenantID    string `json:"tenantId"`
	ActiveLevel int    `json:"activeLevel"`
}

type RiskLevelRule struct {
	ID       string `json:"id"`
	TenantID string `json:"tenantId"`
	Level    int    `json:"level"`
	Keyword  string `json:"keyword"`
	Weight   int    `json:"weight"`
	Enabled  bool   `json:"enabled"`
}

type Sample struct {
	ID             string    `json:"id"`
	CallID         string    `json:"callId"`
	Transcript     string    `json:"transcript"`
	Duration       string    `json:"duration"`
	Origin         string    `json:"origin"`
	Classification string    `json:"classification"`
	Status         string    `json:"status"`
	AudioKey       string    `json:"audioKey,omitempty"`
	ReceivedAt     time.Time `json:"receivedAt"`
}

type Recording struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenantId"`
	OwnerUserID   string    `json:"ownerUserId,omitempty"`
	Phone         string    `json:"phone"`
	Duration      string    `json:"duration"`
	SizeBytes     int64     `json:"size"`
	Verdict       string    `json:"verdict"`
	ObjectKey     string    `json:"objectKey,omitempty"`
	EncryptionKey string    `json:"-"`
	CreatedAt     time.Time `json:"createdAt"`
}

type CallLog struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	UserID    string    `json:"userId,omitempty"`
	Phone     string    `json:"phone"`
	Region    string    `json:"region"`
	Duration  string    `json:"duration"`
	Verdict   string    `json:"verdict"`
	Reason    string    `json:"reason"`
	RiskScore int       `json:"riskScore"`
	CreatedAt time.Time `json:"createdAt"`
}

type VoiceModel struct {
	ID         string    `json:"id"`
	Version    string    `json:"version"`
	Accuracy   float64   `json:"accuracy"`
	SizeBytes  int64     `json:"size"`
	ObjectKey  string    `json:"objectKey,omitempty"`
	Active     bool      `json:"active"`
	UploadedAt time.Time `json:"uploadedAt"`
}

type Device struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	TenantID   *string    `json:"tenantId,omitempty"`
	Type       string     `json:"type"` // enterprise | family
	Status     string     `json:"status"`
	Version    string     `json:"version"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
	Contact    string     `json:"contact"`
}

type AuditLog struct {
	ID       int64     `json:"id"`
	TS       time.Time `json:"ts"`
	ActorID  string    `json:"actorId,omitempty"`
	Action   string    `json:"action"`
	Target   string    `json:"target"`
	Result   string    `json:"result"` // 成功 | 失败
	IP       string    `json:"ip,omitempty"`
}

type Appeal struct {
	ID         string     `json:"id"`
	UserID     string     `json:"userId"`
	TenantID   string     `json:"tenantId"`
	Type       string     `json:"type"`
	Number     string     `json:"number"`
	Reason     string     `json:"reason"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

type EmergencyContact struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Relation  string    `json:"relation"`
	CreatedAt time.Time `json:"createdAt"`
}

type AdminApplication struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Scope     string    `json:"scope"` // family | biz
	Reason    string    `json:"reason"`
	Contact   string    `json:"contact"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
