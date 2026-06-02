import {
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from 'react';
import {
  ChevronUp,
  ChevronDown,
  Copy,
  Eye,
  EyeOff,
  GripVertical,
  Layout,
  MoreHorizontal,
  Sparkles,
  Trash2,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useInlineEditor } from './context';
import type {
  BlockDefinition,
  BlockEditorField,
  BlockSuggestAction,
  BlockSuggestTone,
  SiteDraft,
} from '@/lib/api';
import { cn } from '@/lib/utils';

type DraftBlock = SiteDraft['pages'][number]['blocks'][number];

export function BlockToolbar({
  block,
  definition,
  onDragStart,
  onOpenDetails,
}: {
  block: DraftBlock;
  definition: BlockDefinition | undefined;
  onDragStart?: (event: React.DragEvent) => void;
  onOpenDetails?: () => void;
}) {
  const ctx = useInlineEditor();
  if (!ctx?.enabled) return null;

  const isHidden = Boolean(block.settings?.hidden);
  const selectFields = (definition?.editorSchema ?? []).filter(
    (field) => field.control === 'select' && (field.options?.length ?? 0) > 0,
  );

  return (
    <div
      className="pointer-events-auto absolute -top-[44px] left-1/2 z-40 flex -translate-x-1/2 items-center gap-0.5 rounded-full border border-[oklch(98%_0.005_336_/_0.16)] bg-[oklch(12%_0.018_336_/_0.94)] p-1 shadow-[0_14px_36px_oklch(8%_0.02_336_/_0.55)] backdrop-blur"
      onClick={(event) => event.stopPropagation()}
      data-block-toolbar
    >
      {onDragStart ? (
        <ToolbarButton
          label={`Drag ${definition?.displayName ?? block.type}`}
          draggable
          onDragStart={onDragStart}
        >
          <GripVertical className="size-3.5" aria-hidden />
        </ToolbarButton>
      ) : null}

      <ToolbarSeparator />

      <span className="px-2 text-[10px] font-bold uppercase tracking-[0.14em] text-[oklch(98%_0.005_336_/_0.78)]">
        {definition?.displayName ?? block.type}
      </span>

      <ToolbarSeparator />

      {selectFields.length > 0 ? (
        <VariantSwitcher block={block} fields={selectFields} />
      ) : null}

      {ctx.onSuggestBlock ? (
        <AISuggestMenu />
      ) : null}

      <ToolbarSeparator />

      <ToolbarButton
        label="Move block up"
        disabled={!ctx.canMoveUp}
        onClick={() => void ctx.onMoveBlock(-1)}
      >
        <ChevronUp className="size-3.5" aria-hidden />
      </ToolbarButton>
      <ToolbarButton
        label="Move block down"
        disabled={!ctx.canMoveDown}
        onClick={() => void ctx.onMoveBlock(1)}
      >
        <ChevronDown className="size-3.5" aria-hidden />
      </ToolbarButton>
      <ToolbarButton
        label="Duplicate block"
        onClick={() => void ctx.onDuplicateBlock()}
      >
        <Copy className="size-3.5" aria-hidden />
      </ToolbarButton>
      <ToolbarButton
        label={isHidden ? 'Unhide block' : 'Hide block from preview'}
        onClick={() => void ctx.onToggleHidden(block.id, !isHidden)}
      >
        {isHidden ? (
          <EyeOff className="size-3.5" aria-hidden />
        ) : (
          <Eye className="size-3.5" aria-hidden />
        )}
      </ToolbarButton>
      <ToolbarButton
        label="Delete block"
        onClick={() => void ctx.onDeleteBlock()}
        variant="danger"
      >
        <Trash2 className="size-3.5" aria-hidden />
      </ToolbarButton>

      {onOpenDetails ? (
        <>
          <ToolbarSeparator />
          <ToolbarButton label="Open block details" onClick={onOpenDetails}>
            <MoreHorizontal className="size-3.5" aria-hidden />
            <span className="ml-1 hidden text-[11px] font-bold text-[oklch(98%_0.005_336_/_0.92)] sm:inline">
              Details
            </span>
          </ToolbarButton>
        </>
      ) : null}
    </div>
  );
}

function VariantSwitcher({
  block,
  fields,
}: {
  block: DraftBlock;
  fields: BlockEditorField[];
}) {
  const ctx = useInlineEditor();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function close(event: MouseEvent) {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    function key(event: KeyboardEvent) {
      if (event.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', key);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', key);
    };
  }, [open]);

  const headlineField = fields[0];
  const headlineValue = String(
    (block.props as Record<string, unknown>)?.[headlineField.name] ?? '',
  );

  return (
    <div ref={rootRef} className="relative">
      <ToolbarButton
        label="Change block variant"
        onClick={() => setOpen((current) => !current)}
        aria-expanded={open}
        aria-haspopup="menu"
        active={open}
      >
        <Layout className="size-3.5" aria-hidden />
        <span className="ml-1 hidden text-[11px] font-bold capitalize text-[oklch(98%_0.005_336_/_0.92)] sm:inline">
          {headlineValue || 'Style'}
        </span>
      </ToolbarButton>

      {open ? (
        <div
          role="menu"
          className="absolute left-1/2 top-[calc(100%+8px)] z-50 grid w-[240px] -translate-x-1/2 gap-3 rounded-[14px] border border-border bg-[var(--surface-1)] p-3 shadow-[0_22px_44px_oklch(8%_0.02_336_/_0.55)]"
        >
          {fields.map((field) => {
            const current = String(
              (block.props as Record<string, unknown>)?.[field.name] ?? '',
            );
            return (
              <div key={field.name} className="grid gap-1.5">
                <p className="text-[10px] font-bold uppercase tracking-[0.1em] text-[var(--paper-muted)]">
                  {field.label}
                </p>
                <div className="flex flex-wrap gap-1">
                  {(field.options ?? []).map((option) => {
                    const isActive = option === current;
                    return (
                      <button
                        key={option}
                        type="button"
                        onClick={() => {
                          ctx?.editField(block.id, [field.name], option);
                        }}
                        className={cn(
                          'rounded-full px-2.5 py-1 text-xs font-semibold transition-colors',
                          isActive
                            ? 'bg-[var(--thread-violet)] text-[var(--ink)]'
                            : 'border border-border bg-[var(--surface-2)] text-[var(--paper)] hover:border-[var(--thread-violet)]',
                        )}
                      >
                        {option.replace(/[-_]/g, ' ')}
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

function AISuggestMenu() {
  const ctx = useInlineEditor();
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<'menu' | 'tone' | 'rewrite'>('menu');
  const [instruction, setInstruction] = useState('');
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function close(event: MouseEvent) {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(event.target as Node)) {
        setOpen(false);
        setMode('menu');
      }
    }
    function key(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpen(false);
        setMode('menu');
      }
    }
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', key);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', key);
    };
  }, [open]);

  if (!ctx?.onSuggestBlock) return null;
  const isSuggesting = ctx.isSuggestingBlock ?? false;

  async function runAction(action: BlockSuggestAction, tone?: BlockSuggestTone) {
    await ctx?.onSuggestBlock?.({ action, tone });
    setOpen(false);
    setMode('menu');
  }

  async function runRewrite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = instruction.trim();
    if (!trimmed) return;
    await ctx?.onSuggestBlock?.({ action: 'rewrite', instruction: trimmed });
    setInstruction('');
    setOpen(false);
    setMode('menu');
  }

  return (
    <div ref={rootRef} className="relative">
      <ToolbarButton
        label="Improve with AI"
        onClick={() => {
          setOpen((current) => !current);
          setMode('menu');
        }}
        active={open}
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={isSuggesting}
        accent
      >
        <Sparkles
          className={cn('size-3.5', isSuggesting && 'animate-pulse')}
          aria-hidden
        />
        <span className="ml-1 hidden text-[11px] font-bold text-[var(--thread-gold)] sm:inline">
          {isSuggesting ? 'Improving' : 'Improve'}
        </span>
      </ToolbarButton>

      {open ? (
        <div
          role="menu"
          className="absolute left-1/2 top-[calc(100%+8px)] z-50 w-[280px] -translate-x-1/2 rounded-[14px] border border-border bg-[var(--surface-1)] p-2 shadow-[0_22px_44px_oklch(8%_0.02_336_/_0.55)]"
        >
          {mode === 'menu' ? (
            <div className="grid gap-1">
              <SuggestRow
                label="Tighten"
                description="Shorter, sharper version"
                disabled={isSuggesting}
                onClick={() => void runAction('tighten')}
              />
              <SuggestRow
                label="Expand"
                description="Add useful detail without padding"
                disabled={isSuggesting}
                onClick={() => void runAction('expand')}
              />
              <SuggestRow
                label="Change tone…"
                description="Friendlier, more professional, etc."
                disabled={isSuggesting}
                onClick={() => setMode('tone')}
              />
              <SuggestRow
                label="Rewrite from prompt…"
                description="Describe the change you want"
                disabled={isSuggesting}
                onClick={() => setMode('rewrite')}
              />
            </div>
          ) : null}
          {mode === 'tone' ? (
            <div className="grid gap-1">
              <button
                type="button"
                className="px-2 pb-2 pt-1 text-left text-[10px] uppercase tracking-[0.12em] text-[var(--paper-muted)] hover:text-[var(--paper)]"
                onClick={() => setMode('menu')}
              >
                ← Back
              </button>
              {TONE_OPTIONS.map((tone) => (
                <SuggestRow
                  key={tone.value}
                  label={tone.label}
                  disabled={isSuggesting}
                  onClick={() => void runAction('tone', tone.value)}
                />
              ))}
            </div>
          ) : null}
          {mode === 'rewrite' ? (
            <form className="grid gap-2 p-1" onSubmit={runRewrite}>
              <button
                type="button"
                className="text-left text-[10px] uppercase tracking-[0.12em] text-[var(--paper-muted)] hover:text-[var(--paper)]"
                onClick={() => setMode('menu')}
              >
                ← Back
              </button>
              <Textarea
                rows={3}
                placeholder="Make it more about families with kids"
                value={instruction}
                onChange={(event) => setInstruction(event.target.value)}
                disabled={isSuggesting}
                autoFocus
              />
              <Button
                type="submit"
                size="sm"
                disabled={isSuggesting || instruction.trim().length === 0}
              >
                {isSuggesting ? 'Rewriting…' : 'Rewrite block'}
              </Button>
            </form>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

const TONE_OPTIONS: { value: BlockSuggestTone; label: string }[] = [
  { value: 'friendlier', label: 'Friendlier' },
  { value: 'professional', label: 'More professional' },
  { value: 'playful', label: 'More playful' },
  { value: 'direct', label: 'More direct' },
];

function SuggestRow({
  label,
  description,
  onClick,
  disabled,
}: {
  label: string;
  description?: string;
  onClick: () => void;
  disabled: boolean;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={onClick}
      className="grid w-full gap-0.5 rounded-[10px] px-2 py-2 text-left transition-colors hover:bg-[var(--surface-2)] disabled:cursor-not-allowed disabled:opacity-60"
    >
      <span className="text-sm font-semibold text-[var(--paper)]">{label}</span>
      {description ? (
        <span className="text-xs text-[var(--paper-muted)]">{description}</span>
      ) : null}
    </button>
  );
}

function ToolbarButton({
  label,
  children,
  onClick,
  onDragStart,
  draggable,
  disabled,
  active,
  variant,
  accent,
  ...props
}: {
  label: string;
  children: ReactNode;
  onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void;
  onDragStart?: (event: React.DragEvent) => void;
  draggable?: boolean;
  disabled?: boolean;
  active?: boolean;
  variant?: 'danger';
  accent?: boolean;
  'aria-haspopup'?: 'menu';
  'aria-expanded'?: boolean;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      onDragStart={onDragStart}
      draggable={draggable}
      disabled={disabled}
      className={cn(
        'inline-flex h-7 cursor-pointer items-center justify-center rounded-full px-2 transition-colors',
        active
          ? 'bg-[oklch(98%_0.005_336_/_0.16)] text-[#F9F7F2]'
          : 'text-[oklch(98%_0.005_336_/_0.78)] hover:bg-[oklch(98%_0.005_336_/_0.12)] hover:text-[#F9F7F2]',
        variant === 'danger' &&
          'hover:bg-[color-mix(in_oklch,var(--destructive)_45%,transparent)] hover:text-[#FFEDE8]',
        accent && 'text-[var(--thread-gold)]',
        disabled && 'cursor-not-allowed opacity-40 hover:bg-transparent',
      )}
      {...props}
    >
      {children}
    </button>
  );
}

function ToolbarSeparator() {
  return (
    <span
      aria-hidden="true"
      className="mx-1 h-4 w-px bg-[oklch(98%_0.005_336_/_0.12)]"
    />
  );
}

