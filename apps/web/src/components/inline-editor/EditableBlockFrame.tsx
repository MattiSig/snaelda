import { useState, type ReactNode } from 'react';
import { Eye } from 'lucide-react';
import { useInlineEditor } from './context';
import { BlockToolbar } from './BlockToolbar';
import { BlockDetailsDrawer } from './BlockDetailsDrawer';
import type { BlockDefinition, SiteDraft } from '@/lib/api';
import { cn } from '@/lib/utils';

type DraftBlock = SiteDraft['pages'][number]['blocks'][number];

export function EditableBlockFrame({
  block,
  definition,
  onDragStart,
  children,
}: {
  block: DraftBlock;
  definition: BlockDefinition | undefined;
  onDragStart?: (event: React.DragEvent) => void;
  children: ReactNode;
}) {
  const ctx = useInlineEditor();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const isSelected = ctx?.selectedBlockId === block.id;
  const isHidden = Boolean(block.settings?.hidden);

  if (!ctx?.enabled) {
    return <>{children}</>;
  }

  function handleSelect(event: React.MouseEvent<HTMLDivElement>) {
    if (event.defaultPrevented) return;
    const target = event.target as HTMLElement;
    if (target.closest('[data-inline-edit]')) return;
    if (target.closest('[data-block-toolbar]')) return;
    if (target.closest('button')) return;
    if (target.closest('a')) return;
    ctx?.selectBlock(block.id);
  }

  return (
    <div
      data-canvas-block-id={block.id}
      className={cn(
        'group/block relative isolate transition-[opacity]',
        isHidden && 'opacity-50',
      )}
      onClick={handleSelect}
    >
      <div className="relative">
        <div
          aria-hidden="true"
          className={cn(
            'pointer-events-none absolute inset-x-2 inset-y-2 rounded-[var(--radius-panel)] transition-[box-shadow,opacity] duration-200',
            isSelected
              ? 'opacity-100 shadow-[inset_0_0_0_2px_var(--thread-violet),0_0_0_4px_color-mix(in_oklch,var(--thread-violet)_30%,transparent)]'
              : 'opacity-0 group-hover/block:opacity-100 shadow-[inset_0_0_0_1px_color-mix(in_oklch,var(--thread-violet)_55%,transparent)]',
          )}
        />

        {children}

        {isHidden ? (
          <div className="pointer-events-none absolute right-3 top-3 z-30 flex items-center gap-1.5 rounded-full border border-[oklch(98%_0.005_336_/_0.18)] bg-[oklch(12%_0.018_336_/_0.92)] px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.1em] text-[#F9F7F2] backdrop-blur">
            <Eye className="size-3" aria-hidden />
            Hidden
          </div>
        ) : null}

        {isSelected ? (
          <BlockToolbar
            block={block}
            definition={definition}
            onDragStart={onDragStart}
            onOpenDetails={() => setDrawerOpen(true)}
          />
        ) : null}
      </div>

      {drawerOpen ? (
        <BlockDetailsDrawer
          block={block}
          definition={definition}
          onClose={() => setDrawerOpen(false)}
        />
      ) : null}
    </div>
  );
}
