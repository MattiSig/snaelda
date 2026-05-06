package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SeedDevelopment(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	statements := []string{
		`insert into users (id, email, name)
		 values ('00000000-0000-4000-8000-000000000001', 'demo@snaelda.local', 'Demo User')
		 on conflict (email) do update set name = excluded.name, updated_at = now()`,
		`insert into workspaces (id, name, created_by)
		 values ('00000000-0000-4000-8000-000000000101', 'Demo Workspace', '00000000-0000-4000-8000-000000000001')
		 on conflict (id) do update set name = excluded.name, updated_at = now()`,
		`insert into workspace_members (workspace_id, user_id, role)
		 values ('00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001', 'owner')
		 on conflict (workspace_id, user_id) do update set role = excluded.role`,
		`insert into sites (id, workspace_id, name, slug, status, default_locale, generation_prompt, generation_summary, settings)
		 values (
		 	'00000000-0000-4000-8000-000000000201',
		 	'00000000-0000-4000-8000-000000000101',
		 	'Nordic Studio',
		 	'nordic-studio',
		 	'draft',
		 	'en',
		 	'Create a compact website for a calm Nordic design studio.',
		 	'{"assumptions":["Seeded for local development"]}'::jsonb,
		 	'{}'::jsonb
		 )
		 on conflict (workspace_id, slug) do update
		 set name = excluded.name,
		     generation_prompt = excluded.generation_prompt,
		     generation_summary = excluded.generation_summary,
		     updated_at = now()`,
		`insert into site_domains (id, site_id, hostname, type, status)
		 values ('00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000201', 'nordic-studio.localhost', 'subdomain', 'active')
		 on conflict (hostname) do update set status = excluded.status, updated_at = now()`,
		`insert into themes (id, site_id, name, version, tokens)
		 values (
		 	'00000000-0000-4000-8000-000000000401',
		 	'00000000-0000-4000-8000-000000000201',
		 	'Default',
		 	'theme.v1',
		 	'{"colors":{"background":"#f8f7f4","foreground":"#1d2520","primary":"#315c4f","accent":"#c2774b"},"typography":{"heading":"Inter","body":"Inter"},"layout":{"maxWidth":"1120px"},"shape":{"radius":"8px"}}'::jsonb
		 )
		 on conflict (site_id, name) do update set tokens = excluded.tokens, updated_at = now()`,
		`insert into pages (id, site_id, title, slug, sort_order, status, seo, settings)
		 values (
		 	'00000000-0000-4000-8000-000000000501',
		 	'00000000-0000-4000-8000-000000000201',
		 	'Home',
		 	'/',
		 	0,
		 	'draft',
		 	'{"title":"Nordic Studio","description":"Calm design systems for focused teams."}'::jsonb,
		 	'{}'::jsonb
		 )
		 on conflict (site_id, slug) do update set title = excluded.title, seo = excluded.seo, updated_at = now()`,
		`insert into block_instances (id, page_id, site_id, type, version, sort_order, props, settings, is_hidden)
		 values (
		 	'00000000-0000-4000-8000-000000000601',
		 	'00000000-0000-4000-8000-000000000501',
		 	'00000000-0000-4000-8000-000000000201',
		 	'hero',
		 	'1.0.0',
		 	0,
		 	'{"eyebrow":"Nordic Studio","headline":"Clear websites for focused teams","subheadline":"A seeded draft that proves local Postgres, migrations, and draft persistence tables are ready.","primaryCta":{"label":"Start a project","href":"#contact"},"layout":"centered"}'::jsonb,
		 	'{}'::jsonb,
		 	false
		 )
		 on conflict (id) do update set version = excluded.version, props = excluded.props, updated_at = now()`,
		`insert into block_instances (id, page_id, site_id, type, version, sort_order, props, settings, is_hidden)
		 values (
		 	'00000000-0000-4000-8000-000000000602',
		 	'00000000-0000-4000-8000-000000000501',
		 	'00000000-0000-4000-8000-000000000201',
		 	'text_section',
		 	'1.0.0',
		 	1,
		 	'{"heading":"A structured seed draft","body":"This content is stored as validated application data, not generated code.","alignment":"left","width":"default"}'::jsonb,
		 	'{}'::jsonb,
		 	false
		 )
		 on conflict (id) do update set version = excluded.version, props = excluded.props, updated_at = now()`,
	}

	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("run seed statement: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}

	return nil
}
