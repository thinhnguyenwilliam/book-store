package domain

import "errors"

var ErrForbidden = errors.New("insufficient permissions")
var ErrProtectedRole = errors.New("system role or last super administrator is protected")
var ErrRoleInUse = errors.New("role is still assigned to an account")

type Permission struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
}
type Role struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	System      bool     `json:"system"`
	Permissions []string `json:"permissions"`
}
type Access struct {
	AccountID   string   `json:"account_id"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}
type AuthorizationAudit struct {
	ID        int64  `json:"id"`
	ActorID   string `json:"actor_id"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Before    string `json:"before"`
	After     string `json:"after"`
	TraceID   string `json:"trace_id"`
	CreatedAt string `json:"created_at"`
}
