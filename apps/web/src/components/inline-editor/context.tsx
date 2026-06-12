import { createContext, useContext, type ReactNode } from 'react';
import type {
  AssetRecord,
  BlockBinding,
  BlockDefinition,
  BlockSuggestInput,
  Collection,
  ImageApplyResponse,
  SiteDraft,
} from '@/lib/api';

export type InlineEditorContextValue = {
  enabled: boolean;
  siteId: string;
  selectedBlockId: string | null;
  selectBlock: (blockId: string) => void;
  editField: (blockId: string, path: ReadonlyArray<string | number>, value: unknown) => void;
  assetLibrary: AssetRecord[];
  blockDefinitions: Map<string, BlockDefinition>;
  pages: SiteDraft['pages'];
  collections: Collection[];
  onSuggestBlock?: (input: BlockSuggestInput) => Promise<void>;
  isSuggestingBlock?: boolean;
  onImageApplied?: (response: ImageApplyResponse) => void;
  onMoveBlock: (direction: -1 | 1) => Promise<void> | void;
  onDuplicateBlock: () => Promise<void> | void;
  onDeleteBlock: () => Promise<void> | void;
  onToggleHidden: (blockId: string, hidden: boolean) => Promise<void> | void;
  onUpdateBindings: (
    blockId: string,
    bindings: Record<string, BlockBinding>,
  ) => Promise<void> | void;
  canMoveUp: boolean;
  canMoveDown: boolean;
};

const InlineEditorContext = createContext<InlineEditorContextValue | null>(null);

export function InlineEditorProvider({
  value,
  children,
}: {
  value: InlineEditorContextValue;
  children: ReactNode;
}) {
  return (
    <InlineEditorContext.Provider value={value}>
      {children}
    </InlineEditorContext.Provider>
  );
}

export function useInlineEditor(): InlineEditorContextValue | null {
  return useContext(InlineEditorContext);
}

export function useInlineEditorRequired(): InlineEditorContextValue {
  const ctx = useContext(InlineEditorContext);
  if (!ctx) {
    throw new Error('InlineEditor: missing provider');
  }
  return ctx;
}
