import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, Plus } from 'lucide-react';
import type {
  BlockDefinition,
  BlockEditorField,
} from '@/lib/api';
import { cn } from '@/lib/utils';

const blockTypeIcon: Record<string, string> = {
  hero: '★',
  text_section: '¶',
  image_text: '▣',
  features_grid: '☷',
  gallery: '◫',
  testimonials: '❝',
  pricing_packages: '$',
  cta_band: '➜',
  contact_form: '✉',
  faq: '?',
  team_profile_cards: '👥',
  footer: '⌂',
  stats: '#',
  collection_list: '⇲',
  collection_index: '⛁',
  collection_detail: '⛀',
};

const variantSubtitles: Record<string, Record<string, string>> = {
  hero: {
    standard: 'Classic hero with split or centered layout',
    'full-page': 'Immersive image hero filling the viewport',
    statement: 'Oversized type on the brand color, no image',
  },
  cta_band: {
    primary: 'Quiet, trustworthy invitation',
    accent: 'Loud, accent-colored push',
    secondary: 'Subtle on a tinted surface',
  },
  text_section: {
    left: 'Body text aligned left',
    center: 'Centered, calm reading',
    right: 'Right-aligned, editorial',
  },
};

export function AddBlockInserter({
  index,
  blockRegistry,
  onAdd,
  variant: insertVariant = 'thin',
}: {
  index: number;
  blockRegistry: BlockDefinition[];
  onAdd: (input: {
    blockType: string;
    targetIndex: number;
    initialProps?: Record<string, unknown>;
  }) => Promise<void>;
  variant?: 'thin' | 'standalone';
}) {
  const [open, setOpen] = useState(false);
  const [step, setStep] = useState<'type' | 'variant'>('type');
  const [selectedType, setSelectedType] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  function closePopover() {
    setOpen(false);
    setStep('type');
    setSelectedType(null);
  }

  useEffect(() => {
    if (!open) return;
    function handleDocumentMouseDown(event: MouseEvent) {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(event.target as Node)) {
        closePopover();
      }
    }
    function handleKey(event: KeyboardEvent) {
      if (event.key === 'Escape') closePopover();
    }
    document.addEventListener('mousedown', handleDocumentMouseDown);
    document.addEventListener('keydown', handleKey);
    return () => {
      document.removeEventListener('mousedown', handleDocumentMouseDown);
      document.removeEventListener('keydown', handleKey);
    };
  }, [open]);

  const grouped = groupByCategory(blockRegistry);
  const selectedDefinition = blockRegistry.find(
    (def) => def.type === selectedType,
  );
  const variantAxis = selectedDefinition
    ? pickVariantAxis(selectedDefinition)
    : null;

  async function handleSelectType(type: string) {
    const def = blockRegistry.find((d) => d.type === type);
    if (!def) return;
    const axis = pickVariantAxis(def);
    if (axis) {
      setSelectedType(type);
      setStep('variant');
      return;
    }
    await commit(type);
  }

  async function handleSelectVariant(value: string) {
    if (!selectedDefinition || !variantAxis) return;
    await commit(selectedDefinition.type, { [variantAxis.name]: value });
  }

  async function commit(
    blockType: string,
    overrides?: Record<string, unknown>,
  ) {
    setBusy(true);
    try {
      await onAdd({
        blockType,
        targetIndex: index,
        initialProps: overrides,
      });
      closePopover();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      ref={rootRef}
      className={cn(
        'group/inserter relative isolate flex w-full items-center justify-center',
        insertVariant === 'thin' ? 'py-1' : 'py-6',
        open && 'z-50',
      )}
    >
      {/* Hairline indicator on hover */}
      <span
        aria-hidden="true"
        className={cn(
          'pointer-events-none absolute left-[8%] right-[8%] top-1/2 h-px -translate-y-1/2 transition-colors duration-200',
          open
            ? 'bg-[var(--thread-violet)]'
            : 'bg-transparent group-hover/inserter:bg-[color-mix(in_oklch,var(--thread-violet)_50%,transparent)]',
        )}
      />

      <button
        type="button"
        aria-label="Insert block here"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((value) => !value)}
        disabled={busy}
        className={cn(
          'pointer-events-auto relative z-10 inline-flex h-7 items-center gap-1 rounded-full border border-[oklch(98%_0.005_336_/_0.18)] bg-[oklch(12%_0.018_336_/_0.92)] px-2.5 text-xs font-semibold text-[#F9F7F2] shadow-[0_10px_24px_oklch(8%_0.02_336_/_0.45)] backdrop-blur transition-[opacity,transform,background-color] duration-200',
          open
            ? 'opacity-100 bg-[var(--thread-gold)] text-[var(--ink)] border-transparent'
            : insertVariant === 'standalone'
              ? 'opacity-100 hover:bg-[var(--thread-gold)] hover:text-[var(--ink)] hover:border-transparent'
              : 'opacity-0 group-hover/inserter:opacity-100 hover:bg-[var(--thread-gold)] hover:text-[var(--ink)] hover:border-transparent focus-visible:opacity-100',
          busy && 'cursor-progress opacity-60',
        )}
      >
        <Plus className="size-3.5" aria-hidden />
        <span>Add block</span>
      </button>

      {open ? (
        <div
          role="dialog"
          aria-label="Choose a block to add"
          className="absolute left-1/2 top-[calc(100%+10px)] z-50 w-[min(580px,92vw)] -translate-x-1/2 rounded-[18px] border border-border bg-[var(--surface-1)] p-3 shadow-[0_30px_60px_oklch(8%_0.02_336_/_0.55)]"
        >
          {step === 'type' ? (
            <>
              <header className="flex items-center justify-between gap-3 px-2 pb-2 pt-1">
                <p className="text-[11px] font-bold uppercase tracking-[0.14em] text-[var(--paper-muted)]">
                  Add a block
                </p>
                <span className="text-[11px] text-[var(--paper-muted)]">
                  Step 1 of 2 · Pick a type
                </span>
              </header>
              <div className="grid gap-3 overflow-auto max-h-[60vh] pb-1">
                {Object.entries(grouped).map(([category, definitions]) => (
                  <div key={category} className="grid gap-1.5">
                    <p className="px-2 text-[10px] font-bold uppercase tracking-[0.12em] text-[var(--paper-muted)]">
                      {category}
                    </p>
                    <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-3">
                      {definitions.map((definition) => (
                        <BlockTypeCard
                          key={`${definition.type}@${definition.version}`}
                          definition={definition}
                          disabled={busy}
                          onSelect={() => void handleSelectType(definition.type)}
                        />
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <>
              <header className="flex items-center justify-between gap-3 px-2 pb-2 pt-1">
                <button
                  type="button"
                  onClick={() => setStep('type')}
                  disabled={busy}
                  className="inline-flex items-center gap-1.5 rounded-full px-2 py-1 text-[11px] font-bold uppercase tracking-[0.12em] text-[var(--paper-muted)] transition-colors hover:bg-[var(--surface-2)] hover:text-[var(--paper)] disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <ArrowLeft className="size-3" aria-hidden /> Back
                </button>
                <span className="text-[11px] text-[var(--paper-muted)]">
                  Step 2 of 2 · Pick a style
                </span>
              </header>
              <div className="grid gap-2 p-1">
                <p className="px-2 text-sm text-[var(--paper-muted)]">
                  Choose a {variantAxis?.label.toLowerCase()} for the{' '}
                  <span className="font-semibold text-[var(--paper)]">
                    {selectedDefinition?.displayName}
                  </span>{' '}
                  block.
                </p>
                <div className="grid gap-1.5">
                  {(variantAxis?.options ?? []).map((option) => {
                    const subtitle =
                      variantSubtitles[selectedDefinition?.type ?? '']?.[
                        option
                      ];
                    return (
                      <button
                        key={option}
                        type="button"
                        disabled={busy}
                        onClick={() => void handleSelectVariant(option)}
                        className="grid w-full grid-cols-[1fr_auto] items-center gap-3 rounded-[12px] border border-border bg-[var(--surface-2)] px-3 py-2.5 text-left transition-colors hover:border-[var(--thread-violet)] hover:bg-[var(--surface-3)] disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        <span className="grid gap-0.5">
                          <span className="text-sm font-semibold capitalize text-[var(--paper)]">
                            {option.replace(/[-_]/g, ' ')}
                          </span>
                          {subtitle ? (
                            <span className="text-xs text-[var(--paper-muted)]">
                              {subtitle}
                            </span>
                          ) : null}
                        </span>
                        <span
                          aria-hidden="true"
                          className="text-[var(--thread-violet)]"
                        >
                          →
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}

function BlockTypeCard({
  definition,
  onSelect,
  disabled,
}: {
  definition: BlockDefinition;
  onSelect: () => void;
  disabled?: boolean;
}) {
  const icon = blockTypeIcon[definition.type] ?? '⊞';
  return (
    <button
      type="button"
      onClick={onSelect}
      disabled={disabled}
      className="group grid grid-cols-[auto_minmax(0,1fr)] items-center gap-2 rounded-[12px] border border-border bg-[var(--surface-2)] px-2.5 py-2 text-left transition-[border-color,transform] hover:-translate-y-px hover:border-[var(--thread-violet)] disabled:cursor-not-allowed disabled:opacity-60"
    >
      <span className="flex size-8 shrink-0 items-center justify-center rounded-[8px] border border-border bg-[var(--surface-1)] text-base text-[var(--paper)] group-hover:border-[var(--thread-violet)] group-hover:bg-[color-mix(in_oklch,var(--thread-violet)_18%,var(--surface-1))]">
        {icon}
      </span>
      <span className="grid min-w-0 gap-0.5">
        <span className="truncate text-sm font-semibold text-[var(--paper)]">
          {definition.displayName}
        </span>
        <span className="truncate text-[10px] uppercase tracking-[0.1em] text-[var(--paper-muted)]">
          {definition.category}
        </span>
      </span>
    </button>
  );
}

function groupByCategory(
  registry: BlockDefinition[],
): Record<string, BlockDefinition[]> {
  const groups: Record<string, BlockDefinition[]> = {};
  for (const def of registry) {
    const category = def.category || 'Other';
    if (!groups[category]) groups[category] = [];
    groups[category].push(def);
  }
  return groups;
}

function pickVariantAxis(definition: BlockDefinition): BlockEditorField | null {
  const fields = definition.editorSchema ?? [];
  const named =
    fields.find((field) => field.name === 'variant' && field.control === 'select') ??
    fields.find((field) => field.name === 'layout' && field.control === 'select') ??
    fields.find((field) => field.name === 'alignment' && field.control === 'select');
  if (named && (named.options?.length ?? 0) > 1) return named;
  const firstSelect = fields.find(
    (field) => field.control === 'select' && (field.options?.length ?? 0) > 1,
  );
  return firstSelect ?? null;
}
