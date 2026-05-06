package authorization

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MattiSig/snaelda/internal/auth"
	"github.com/jackc/pgx/v5"
)

type fakeStore struct {
	scope Scope
	err   error
	sql   string
	args  []any
}

func (s *fakeStore) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	s.sql = sql
	s.args = args
	return fakeRow{scope: s.scope, err: s.err}
}

type fakeRow struct {
	scope Scope
	err   error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}

	values := []string{
		r.scope.WorkspaceID,
		r.scope.SiteID,
		r.scope.PageID,
		r.scope.BlockID,
		r.scope.AssetID,
		r.scope.Role,
	}
	for index, value := range values {
		target := dest[index].(*string)
		*target = value
	}
	return nil
}

func TestRequireResourceScopes(t *testing.T) {
	ctx := userContext()

	tests := []struct {
		name        string
		resourceID  string
		tableMarker string
		scope       Scope
		require     func(*Authorizer, context.Context, string) (Scope, error)
	}{
		{
			name:        "workspace",
			resourceID:  "workspace-1",
			tableMarker: "from workspace_members",
			scope:       Scope{WorkspaceID: "workspace-1", Role: RoleOwner},
			require: func(a *Authorizer, ctx context.Context, id string) (Scope, error) {
				return a.RequireWorkspaceMember(ctx, id)
			},
		},
		{
			name:        "site",
			resourceID:  "site-1",
			tableMarker: "from sites",
			scope:       Scope{WorkspaceID: "workspace-1", SiteID: "site-1", Role: RoleEditor},
			require: func(a *Authorizer, ctx context.Context, id string) (Scope, error) {
				return a.RequireSite(ctx, id)
			},
		},
		{
			name:        "page",
			resourceID:  "page-1",
			tableMarker: "from pages",
			scope:       Scope{WorkspaceID: "workspace-1", SiteID: "site-1", PageID: "page-1", Role: RoleEditor},
			require: func(a *Authorizer, ctx context.Context, id string) (Scope, error) {
				return a.RequirePage(ctx, id)
			},
		},
		{
			name:        "block",
			resourceID:  "block-1",
			tableMarker: "from block_instances",
			scope:       Scope{WorkspaceID: "workspace-1", SiteID: "site-1", PageID: "page-1", BlockID: "block-1", Role: RoleEditor},
			require: func(a *Authorizer, ctx context.Context, id string) (Scope, error) {
				return a.RequireBlock(ctx, id)
			},
		},
		{
			name:        "asset",
			resourceID:  "asset-1",
			tableMarker: "from assets",
			scope:       Scope{WorkspaceID: "workspace-1", SiteID: "site-1", AssetID: "asset-1", Role: RoleEditor},
			require: func(a *Authorizer, ctx context.Context, id string) (Scope, error) {
				return a.RequireAsset(ctx, id)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{scope: tt.scope}
			authorizer := New(store)

			scope, err := tt.require(authorizer, ctx, tt.resourceID)
			if err != nil {
				t.Fatalf("expected resource access, got %v", err)
			}
			if scope != tt.scope {
				t.Fatalf("expected scope %#v, got %#v", tt.scope, scope)
			}
			if len(store.args) != 2 || store.args[0] != tt.resourceID || store.args[1] != "user-1" {
				t.Fatalf("expected resource id and user id args, got %#v", store.args)
			}
			if !strings.Contains(store.sql, tt.tableMarker) {
				t.Fatalf("expected query to contain %q, got %q", tt.tableMarker, store.sql)
			}
		})
	}
}

func TestRequireResourceRejectsUnknownOwnership(t *testing.T) {
	authorizer := New(&fakeStore{err: pgx.ErrNoRows})

	_, err := authorizer.RequireSite(userContext(), "site-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestRequireResourceRejectsDisallowedRole(t *testing.T) {
	authorizer := New(&fakeStore{
		scope: Scope{WorkspaceID: "workspace-1", SiteID: "site-1", Role: RoleViewer},
	})

	_, err := authorizer.RequireSite(userContext(), "site-1", RoleOwner, RoleEditor)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestRequireResourceRequiresAuthenticatedUser(t *testing.T) {
	authorizer := New(&fakeStore{})

	_, err := authorizer.RequireSite(context.Background(), "site-1")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated error, got %v", err)
	}
}

func TestRequireResourceRequiresStore(t *testing.T) {
	authorizer := New(nil)

	_, err := authorizer.RequireSite(userContext(), "site-1")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestRequireResourceRequiresID(t *testing.T) {
	authorizer := New(&fakeStore{})

	_, err := authorizer.RequireSite(userContext(), " ")
	if !errors.Is(err, ErrInvalidResourceID) {
		t.Fatalf("expected invalid resource id error, got %v", err)
	}
}

func userContext() context.Context {
	return auth.WithUser(context.Background(), auth.User{
		ID:            "user-1",
		Email:         "demo@snaelda.local",
		WorkspaceID:   "workspace-1",
		WorkspaceRole: RoleOwner,
	})
}
