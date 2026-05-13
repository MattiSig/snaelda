import type { SiteDraft } from '@/lib/api'

type DraftPage = SiteDraft['pages'][number]
type DraftBlock = DraftPage['blocks'][number]

export type EditorCanvasBlock = {
  block: DraftBlock
  originalIndex: number
}

export type EditorCanvasPage = {
  page: DraftPage
  visibleBlocks: EditorCanvasBlock[]
  hiddenBlocks: EditorCanvasBlock[]
}

export function draftToEditorCanvasPage(
  draft: SiteDraft,
  pageId: string | null | undefined,
): EditorCanvasPage | null {
  const page =
    draft.pages.find((candidate) => candidate.id === pageId) ??
    draft.pages[0] ??
    null
  if (!page) {
    return null
  }

  const visibleBlocks: EditorCanvasBlock[] = []
  const hiddenBlocks: EditorCanvasBlock[] = []

  page.blocks.forEach((block, originalIndex) => {
    const canvasBlock = { block, originalIndex }
    if (block.settings?.hidden) {
      hiddenBlocks.push(canvasBlock)
      return
    }
    visibleBlocks.push(canvasBlock)
  })

  return {
    page,
    visibleBlocks,
    hiddenBlocks,
  }
}

export function reorderEditorCanvasBlocks(
  blocks: EditorCanvasBlock[],
  sourceBlockId: string,
  targetIndex: number,
): string[] | null {
  const sourceIndex = blocks.findIndex(({ block }) => block.id === sourceBlockId)
  if (sourceIndex === -1) {
    return null
  }

  const boundedTargetIndex = Math.max(0, Math.min(targetIndex, blocks.length))
  const reordered = [...blocks]
  const [moved] = reordered.splice(sourceIndex, 1)
  const insertAt =
    sourceIndex < boundedTargetIndex
      ? Math.max(0, boundedTargetIndex - 1)
      : boundedTargetIndex
  reordered.splice(insertAt, 0, moved)

  return reordered.map(({ block }) => block.id)
}

export function insertEditorCanvasBlock(
  blocks: EditorCanvasBlock[],
  blockId: string,
  targetIndex: number,
): string[] {
  const ordered = [...blocks]
  const boundedTargetIndex = Math.max(0, Math.min(targetIndex, ordered.length))
  ordered.splice(boundedTargetIndex, 0, {
    block: { id: blockId } as DraftBlock,
    originalIndex: boundedTargetIndex,
  })
  return ordered.map(({ block }) => block.id)
}

export function buildCanonicalBlockOrder(
  editorPage: EditorCanvasPage,
  visibleBlockIds: string[],
): string[] {
  const hiddenBlockIDs = editorPage.hiddenBlocks.map(({ block }) => block.id)
  return [...visibleBlockIds, ...hiddenBlockIDs]
}
