package authorization

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/jackc/pgx/v5"
)

const (
	RoleOwner  = "owner"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

var (
	ErrForbidden         = errors.New("authorization forbidden")
	ErrInvalidResourceID = errors.New("resource id is required")
	ErrUnauthenticated   = errors.New("authenticated user is required")
	ErrUnavailable       = errors.New("authorization store is not configured")
)

type Store interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Authorizer struct {
	store Store
}

type Scope struct {
	WorkspaceID string
	SiteID      string
	PageID      string
	BlockID     string
	AssetID     string
	Role        string
}

func New(store Store) *Authorizer {
	return &Authorizer{store: store}
}

func RequireUser(ctx context.Context) (auth.User, error) {
	user, ok := auth.UserFromContext(ctx)
	if !ok || user.ID == "" {
		return auth.User{}, ErrUnauthenticated
	}
	return user, nil
}

func (a *Authorizer) RequireWorkspaceMember(ctx context.Context, workspaceID string, allowedRoles ...string) (Scope, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return Scope{}, ErrInvalidResourceID
	}

	user, err := RequireUser(ctx)
	if err != nil {
		return Scope{}, err
	}

	return a.requireScope(ctx, "workspace", `
		select wm.workspace_id::text,
		       ''::text as site_id,
		       ''::text as page_id,
		       ''::text as block_id,
		       ''::text as asset_id,
		       wm.role
		from workspace_members wm
		where wm.workspace_id = $1
		  and wm.user_id = $2
	`, allowedRoles, workspaceID, user.ID)
}

func (a *Authorizer) RequireSite(ctx context.Context, siteID string, allowedRoles ...string) (Scope, error) {
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return Scope{}, ErrInvalidResourceID
	}

	user, err := RequireUser(ctx)
	if err != nil {
		return Scope{}, err
	}

	return a.requireScope(ctx, "site", `
		select s.workspace_id::text,
		       s.id::text as site_id,
		       ''::text as page_id,
		       ''::text as block_id,
		       ''::text as asset_id,
		       wm.role
		from sites s
		join workspace_members wm on wm.workspace_id = s.workspace_id
		where s.id = $1
		  and wm.user_id = $2
	`, allowedRoles, siteID, user.ID)
}

func (a *Authorizer) RequirePage(ctx context.Context, pageID string, allowedRoles ...string) (Scope, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return Scope{}, ErrInvalidResourceID
	}

	user, err := RequireUser(ctx)
	if err != nil {
		return Scope{}, err
	}

	return a.requireScope(ctx, "page", `
		select s.workspace_id::text,
		       s.id::text as site_id,
		       p.id::text as page_id,
		       ''::text as block_id,
		       ''::text as asset_id,
		       wm.role
		from pages p
		join sites s on s.id = p.site_id
		join workspace_members wm on wm.workspace_id = s.workspace_id
		where p.id = $1
		  and wm.user_id = $2
	`, allowedRoles, pageID, user.ID)
}

func (a *Authorizer) RequireBlock(ctx context.Context, blockID string, allowedRoles ...string) (Scope, error) {
	blockID = strings.TrimSpace(blockID)
	if blockID == "" {
		return Scope{}, ErrInvalidResourceID
	}

	user, err := RequireUser(ctx)
	if err != nil {
		return Scope{}, err
	}

	return a.requireScope(ctx, "block", `
		select s.workspace_id::text,
		       s.id::text as site_id,
		       b.page_id::text,
		       b.id::text as block_id,
		       ''::text as asset_id,
		       wm.role
		from block_instances b
		join sites s on s.id = b.site_id
		join workspace_members wm on wm.workspace_id = s.workspace_id
		where b.id = $1
		  and wm.user_id = $2
	`, allowedRoles, blockID, user.ID)
}

func (a *Authorizer) RequireAsset(ctx context.Context, assetID string, allowedRoles ...string) (Scope, error) {
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return Scope{}, ErrInvalidResourceID
	}

	user, err := RequireUser(ctx)
	if err != nil {
		return Scope{}, err
	}

	return a.requireScope(ctx, "asset", `
		select a.workspace_id::text,
		       coalesce(a.site_id::text, '') as site_id,
		       ''::text as page_id,
		       ''::text as block_id,
		       a.id::text as asset_id,
		       wm.role
		from assets a
		join workspace_members wm on wm.workspace_id = a.workspace_id
		where a.id = $1
		  and wm.user_id = $2
	`, allowedRoles, assetID, user.ID)
}

func (a *Authorizer) requireScope(ctx context.Context, resource string, sql string, allowedRoles []string, args ...any) (Scope, error) {
	if a == nil || a.store == nil {
		return Scope{}, ErrUnavailable
	}

	var scope Scope
	err := a.store.QueryRow(ctx, sql, args...).Scan(
		&scope.WorkspaceID,
		&scope.SiteID,
		&scope.PageID,
		&scope.BlockID,
		&scope.AssetID,
		&scope.Role,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Scope{}, ErrForbidden
	}
	if err != nil {
		return Scope{}, fmt.Errorf("authorize %s: %w", resource, err)
	}
	if !roleAllowed(scope.Role, allowedRoles) {
		return Scope{}, ErrForbidden
	}
	return scope, nil
}

func roleAllowed(role string, allowedRoles []string) bool {
	if len(allowedRoles) == 0 {
		return true
	}
	for _, allowedRole := range allowedRoles {
		if role == allowedRole {
			return true
		}
	}
	return false
}
