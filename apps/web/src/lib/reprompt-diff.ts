import type {
  DraftRevisionRecord,
  RepromptHistoryRecord,
  SiteDraft,
} from './api'

export type DiffStatus = 'added' | 'removed' | 'modified' | 'unchanged'

export type DiffField = {
  key: string
  before?: string
  after?: string
}

export type BlockDiff = {
  key: string
  status: DiffStatus
  beforeBlock?: SiteDraft['pages'][number]['blocks'][number]
  afterBlock?: SiteDraft['pages'][number]['blocks'][number]
  fields: DiffField[]
}

export type PageDiff = {
  key: string
  title: string
  status: DiffStatus
  beforePage?: SiteDraft['pages'][number]
  afterPage?: SiteDraft['pages'][number]
  blocks: BlockDiff[]
}

export function buildRepromptDiff(
  reprompt: RepromptHistoryRecord,
  previousRevision: DraftRevisionRecord,
  resultRevision: DraftRevisionRecord,
): PageDiff[] {
  const pagePairs = pairPages(
    selectPages(previousRevision.draft, reprompt),
    selectPages(resultRevision.draft, reprompt),
  )
  const pageDiffs: PageDiff[] = []

  for (let index = 0; index < pagePairs.length; index += 1) {
    const pair = pagePairs[index]
    const beforePage = pair.beforePage
    const afterPage = pair.afterPage
    const blocks = buildBlockDiffs(beforePage, afterPage)
    const status = derivePageStatus(beforePage, afterPage, blocks)

    if (status === 'unchanged') {
      continue
    }

    pageDiffs.push({
      key: pageDiffKey(beforePage, afterPage, index),
      title:
        afterPage?.title || beforePage?.title || `Page ${String(index + 1)}`,
      status,
      beforePage,
      afterPage,
      blocks,
    })
  }

  return pageDiffs
}

function selectPages(
  draft: SiteDraft,
  reprompt: RepromptHistoryRecord,
): SiteDraft['pages'] {
  if (reprompt.scope === 'page' && reprompt.targetId) {
    return draft.pages.filter((page) => page.id === reprompt.targetId)
  }
  return draft.pages
}

function pairPages(
  beforePages: SiteDraft['pages'],
  afterPages: SiteDraft['pages'],
) {
  const byKey = new Map<
    string,
    {
      beforePage?: SiteDraft['pages'][number]
      afterPage?: SiteDraft['pages'][number]
      order: number
    }
  >()

  beforePages.forEach((page, index) => {
    const key = pagePairKey(page, index)
    byKey.set(key, {
      beforePage: page,
      afterPage: byKey.get(key)?.afterPage,
      order: index,
    })
  })

  afterPages.forEach((page, index) => {
    const key = pagePairKey(page, index)
    const existing = byKey.get(key)
    byKey.set(key, {
      beforePage: existing?.beforePage,
      afterPage: page,
      order: existing?.order ?? index,
    })
  })

  return [...byKey.values()].sort((left, right) => left.order - right.order)
}

function buildBlockDiffs(
  beforePage?: SiteDraft['pages'][number],
  afterPage?: SiteDraft['pages'][number],
): BlockDiff[] {
  const beforeBlocks = beforePage?.blocks ?? []
  const afterBlocks = afterPage?.blocks ?? []
  const count = Math.max(beforeBlocks.length, afterBlocks.length)
  const diffs: BlockDiff[] = []

  for (let index = 0; index < count; index += 1) {
    const beforeBlock = beforeBlocks[index]
    const afterBlock = afterBlocks[index]
    const status = deriveBlockStatus(beforeBlock, afterBlock)
    if (status === 'unchanged') {
      continue
    }
    diffs.push({
      key:
        afterBlock?.id ||
        beforeBlock?.id ||
        `block-${String(index + 1)}-${beforeBlock?.type || afterBlock?.type || 'unknown'}`,
      status,
      beforeBlock,
      afterBlock,
      fields: diffFields(beforeBlock, afterBlock),
    })
  }

  return diffs
}

function derivePageStatus(
  beforePage: SiteDraft['pages'][number] | undefined,
  afterPage: SiteDraft['pages'][number] | undefined,
  blocks: BlockDiff[],
): DiffStatus {
  if (!beforePage && afterPage) {
    return 'added'
  }
  if (beforePage && !afterPage) {
    return 'removed'
  }
  if (!beforePage || !afterPage) {
    return 'unchanged'
  }
  if (
    beforePage.title !== afterPage.title ||
    beforePage.slug !== afterPage.slug ||
    blocks.length > 0
  ) {
    return 'modified'
  }
  return 'unchanged'
}

function deriveBlockStatus(
  beforeBlock?: SiteDraft['pages'][number]['blocks'][number],
  afterBlock?: SiteDraft['pages'][number]['blocks'][number],
): DiffStatus {
  if (!beforeBlock && afterBlock) {
    return 'added'
  }
  if (beforeBlock && !afterBlock) {
    return 'removed'
  }
  if (!beforeBlock || !afterBlock) {
    return 'unchanged'
  }
  if (
    beforeBlock.type !== afterBlock.type ||
    JSON.stringify(beforeBlock.props) !== JSON.stringify(afterBlock.props)
  ) {
    return 'modified'
  }
  return 'unchanged'
}

function diffFields(
  beforeBlock?: SiteDraft['pages'][number]['blocks'][number],
  afterBlock?: SiteDraft['pages'][number]['blocks'][number],
): DiffField[] {
  const fields: DiffField[] = []
  if (beforeBlock?.type !== afterBlock?.type) {
    fields.push({
      key: 'type',
      before: beforeBlock?.type,
      after: afterBlock?.type,
    })
  }

  const keys = new Set([
    ...Object.keys(beforeBlock?.props ?? {}),
    ...Object.keys(afterBlock?.props ?? {}),
  ])

  for (const key of keys) {
    const beforeValue = beforeBlock?.props?.[key]
    const afterValue = afterBlock?.props?.[key]
    if (JSON.stringify(beforeValue) === JSON.stringify(afterValue)) {
      continue
    }
    fields.push({
      key,
      before: formatDiffValue(beforeValue),
      after: formatDiffValue(afterValue),
    })
  }

  return fields
}

function formatDiffValue(value: unknown) {
  if (value === undefined) {
    return undefined
  }
  if (value === null) {
    return 'null'
  }
  if (typeof value === 'string') {
    return value
  }
  if (
    typeof value === 'number' ||
    typeof value === 'boolean' ||
    typeof value === 'bigint'
  ) {
    return String(value)
  }
  return JSON.stringify(value, null, 2)
}

function pageDiffKey(
  beforePage: SiteDraft['pages'][number] | undefined,
  afterPage: SiteDraft['pages'][number] | undefined,
  index: number,
) {
  return (
    afterPage?.slug ||
    beforePage?.slug ||
    afterPage?.id ||
    beforePage?.id ||
    `page-${String(index + 1)}`
  )
}

function pagePairKey(page: SiteDraft['pages'][number], index: number) {
  return page.slug || page.title || `${page.id}-${String(index)}`
}
