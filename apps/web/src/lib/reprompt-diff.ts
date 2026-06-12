import type {
  DraftRevisionRecord,
  RepromptHistoryRecord,
  SiteDraft,
} from './api'

export type DiffStatus = 'added' | 'removed' | 'modified' | 'unchanged'

export type DiffTextPart = {
  value: string
  changed: boolean
}

export type DiffField = {
  key: string
  before?: string
  after?: string
  beforeParts: DiffTextPart[]
  afterParts: DiffTextPart[]
}

export type BlockDiff = {
  key: string
  status: DiffStatus
  beforeBlock?: SiteDraft['pages'][number]['blocks'][number]
  afterBlock?: SiteDraft['pages'][number]['blocks'][number]
  beforeIndex: number
  afterIndex: number
  summary: string
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
    selectPages(previousRevision, reprompt),
    selectPages(resultRevision, reprompt),
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
  revision: DraftRevisionRecord,
  reprompt: RepromptHistoryRecord,
): SiteDraft['pages'] {
  if (reprompt.scope === 'page' && reprompt.targetId) {
    return revision.draft.pages.filter((page) => page.id === reprompt.targetId)
  }

  if (reprompt.scope === 'block') {
    const pageId = revision.targetId
    if (pageId) {
      return revision.draft.pages.filter((page) => page.id === pageId)
    }
    if (reprompt.targetId) {
      return revision.draft.pages.filter((page) =>
        page.blocks.some((block) => block.id === reprompt.targetId),
      )
    }
  }

  return revision.draft.pages
}

function pairPages(
  beforePages: SiteDraft['pages'],
  afterPages: SiteDraft['pages'],
) {
  const pairs: Array<{
    beforePage?: SiteDraft['pages'][number]
    afterPage?: SiteDraft['pages'][number]
    order: number
  }> = []
  const usedAfter = new Set<number>()

  beforePages.forEach((beforePage, index) => {
    const afterIndex = afterPages.findIndex(
      (afterPage, candidateIndex) =>
        !usedAfter.has(candidateIndex) && pagesMatch(beforePage, afterPage),
    )

    if (afterIndex >= 0) {
      usedAfter.add(afterIndex)
      pairs.push({
        beforePage,
        afterPage: afterPages[afterIndex],
        order: index,
      })
      return
    }

    pairs.push({
      beforePage,
      order: index,
    })
  })

  afterPages.forEach((afterPage, index) => {
    if (usedAfter.has(index)) {
      return
    }
    pairs.push({
      afterPage,
      order: beforePages.length + index,
    })
  })

  return pairs.sort((left, right) => left.order - right.order)
}

function pagesMatch(
  beforePage: SiteDraft['pages'][number],
  afterPage: SiteDraft['pages'][number],
) {
  if (beforePage.id && afterPage.id && beforePage.id === afterPage.id) {
    return true
  }
  if (beforePage.slug && afterPage.slug && beforePage.slug === afterPage.slug) {
    return true
  }
  return normalizeLabel(beforePage.title) === normalizeLabel(afterPage.title)
}

function buildBlockDiffs(
  beforePage?: SiteDraft['pages'][number],
  afterPage?: SiteDraft['pages'][number],
): BlockDiff[] {
  const pairs = pairBlocks(beforePage?.blocks ?? [], afterPage?.blocks ?? [])
  const diffs: BlockDiff[] = []

  for (const pair of pairs) {
    const status = deriveBlockStatus(pair.beforeBlock, pair.afterBlock)
    if (status === 'unchanged') {
      continue
    }

    diffs.push({
      key:
        pair.afterBlock?.id ||
        pair.beforeBlock?.id ||
        `block-${String(pair.beforeIndex + pair.afterIndex + 2)}`,
      status,
      beforeBlock: pair.beforeBlock,
      afterBlock: pair.afterBlock,
      beforeIndex: pair.beforeIndex,
      afterIndex: pair.afterIndex,
      summary: summarizeBlock(pair.afterBlock ?? pair.beforeBlock),
      fields: diffFields(pair.beforeBlock, pair.afterBlock),
    })
  }

  return diffs
}

function pairBlocks(
  beforeBlocks: SiteDraft['pages'][number]['blocks'],
  afterBlocks: SiteDraft['pages'][number]['blocks'],
) {
  const gapPenalty = -48
  const rows = beforeBlocks.length
  const cols = afterBlocks.length
  const scores = Array.from({ length: rows + 1 }, () =>
    Array.from({ length: cols + 1 }, () => 0),
  )
  const steps = Array.from({ length: rows + 1 }, () =>
    Array.from(
      { length: cols + 1 },
      (): 'match' | 'before-gap' | 'after-gap' | null => null,
    ),
  )

  for (let row = 1; row <= rows; row += 1) {
    scores[row][0] = row * gapPenalty
    steps[row][0] = 'before-gap'
  }
  for (let col = 1; col <= cols; col += 1) {
    scores[0][col] = col * gapPenalty
    steps[0][col] = 'after-gap'
  }

  for (let row = 1; row <= rows; row += 1) {
    for (let col = 1; col <= cols; col += 1) {
      const beforeBlock = beforeBlocks[row - 1]
      const afterBlock = afterBlocks[col - 1]
      const matchScore =
        scores[row - 1][col - 1] + blockSimilarity(beforeBlock, afterBlock)
      const beforeGapScore = scores[row - 1][col] + gapPenalty
      const afterGapScore = scores[row][col - 1] + gapPenalty

      if (matchScore >= beforeGapScore && matchScore >= afterGapScore) {
        scores[row][col] = matchScore
        steps[row][col] = 'match'
      } else if (beforeGapScore >= afterGapScore) {
        scores[row][col] = beforeGapScore
        steps[row][col] = 'before-gap'
      } else {
        scores[row][col] = afterGapScore
        steps[row][col] = 'after-gap'
      }
    }
  }

  const pairs: Array<{
    beforeBlock?: SiteDraft['pages'][number]['blocks'][number]
    afterBlock?: SiteDraft['pages'][number]['blocks'][number]
    beforeIndex: number
    afterIndex: number
  }> = []

  let row = rows
  let col = cols
  while (row > 0 || col > 0) {
    const step = steps[row][col]
    if (step === 'match' && row > 0 && col > 0) {
      pairs.push({
        beforeBlock: beforeBlocks[row - 1],
        afterBlock: afterBlocks[col - 1],
        beforeIndex: row - 1,
        afterIndex: col - 1,
      })
      row -= 1
      col -= 1
      continue
    }
    if (step === 'before-gap' && row > 0) {
      pairs.push({
        beforeBlock: beforeBlocks[row - 1],
        beforeIndex: row - 1,
        afterIndex: col,
      })
      row -= 1
      continue
    }
    if (col > 0) {
      pairs.push({
        afterBlock: afterBlocks[col - 1],
        beforeIndex: row,
        afterIndex: col - 1,
      })
      col -= 1
      continue
    }
    if (row > 0) {
      pairs.push({
        beforeBlock: beforeBlocks[row - 1],
        beforeIndex: row - 1,
        afterIndex: col,
      })
      row -= 1
    }
  }

  return pairs.reverse()
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
  if (serializeBlock(beforeBlock) !== serializeBlock(afterBlock)) {
    return 'modified'
  }
  return 'unchanged'
}

function blockSimilarity(
  beforeBlock: SiteDraft['pages'][number]['blocks'][number],
  afterBlock: SiteDraft['pages'][number]['blocks'][number],
) {
  if (beforeBlock.id === afterBlock.id) {
    return 1_000
  }

  if (beforeBlock.type !== afterBlock.type) {
    return -80
  }

  let score = 40
  if (beforeBlock.version === afterBlock.version) {
    score += 10
  }
  if (serializeBlock(beforeBlock) === serializeBlock(afterBlock)) {
    score += 220
  }

  const beforeSummary = summarizeBlock(beforeBlock)
  const afterSummary = summarizeBlock(afterBlock)
  if (beforeSummary && afterSummary) {
    if (beforeSummary === afterSummary) {
      score += 160
    } else {
      score += Math.round(tokenOverlap(beforeSummary, afterSummary) * 90)
    }
  }

  const beforeKeys = Object.keys(beforeBlock.props ?? {})
  const afterKeys = Object.keys(afterBlock.props ?? {})
  const keyOverlap = overlapRatio(beforeKeys, afterKeys)
  score += Math.round(keyOverlap * 30)

  return score
}

function diffFields(
  beforeBlock?: SiteDraft['pages'][number]['blocks'][number],
  afterBlock?: SiteDraft['pages'][number]['blocks'][number],
): DiffField[] {
  const fields: DiffField[] = []
  if (beforeBlock?.type !== afterBlock?.type) {
    fields.push(buildDiffField('type', beforeBlock?.type, afterBlock?.type))
  }

  if (beforeBlock?.version !== afterBlock?.version) {
    fields.push(
      buildDiffField('version', beforeBlock?.version, afterBlock?.version),
    )
  }

  const keys = new Set([
    ...Object.keys(beforeBlock?.props ?? {}),
    ...Object.keys(afterBlock?.props ?? {}),
  ])

  for (const key of keys) {
    const beforeValue = beforeBlock?.props?.[key]
    const afterValue = afterBlock?.props?.[key]
    if (serializeValue(beforeValue) === serializeValue(afterValue)) {
      continue
    }
    fields.push(
      buildDiffField(
        key,
        formatDiffValue(beforeValue),
        formatDiffValue(afterValue),
      ),
    )
  }

  if (beforeBlock?.settings?.hidden !== afterBlock?.settings?.hidden) {
    fields.push(
      buildDiffField(
        'hidden',
        beforeBlock?.settings?.hidden ? 'Hidden' : 'Visible',
        afterBlock?.settings?.hidden ? 'Hidden' : 'Visible',
      ),
    )
  }

  if (
    serializeValue(beforeBlock?.bindings) !== serializeValue(afterBlock?.bindings)
  ) {
    fields.push(
      buildDiffField(
        'bindings',
        formatDiffValue(beforeBlock?.bindings),
        formatDiffValue(afterBlock?.bindings),
      ),
    )
  }

  return fields
}

function buildDiffField(key: string, before?: string, after?: string): DiffField {
  const parts = buildTextDiffParts(before, after)
  return {
    key,
    before,
    after,
    beforeParts: parts.beforeParts,
    afterParts: parts.afterParts,
  }
}

function buildTextDiffParts(before?: string, after?: string) {
  if (!before && !after) {
    return { beforeParts: [], afterParts: [] }
  }
  if (before === after) {
    return {
      beforeParts: before ? [{ value: before, changed: false }] : [],
      afterParts: after ? [{ value: after, changed: false }] : [],
    }
  }

  const beforeValue = before ?? ''
  const afterValue = after ?? ''
  const prefixLength = sharedPrefixLength(beforeValue, afterValue)
  const suffixLength = sharedSuffixLength(beforeValue, afterValue, prefixLength)

  const beforeParts = toDiffParts(
    beforeValue,
    prefixLength,
    suffixLength,
    before === undefined,
  )
  const afterParts = toDiffParts(
    afterValue,
    prefixLength,
    suffixLength,
    after === undefined,
  )

  return { beforeParts, afterParts }
}

function toDiffParts(
  value: string,
  prefixLength: number,
  suffixLength: number,
  isMissing: boolean,
) {
  if (isMissing) {
    return []
  }

  const parts: DiffTextPart[] = []
  const changedEnd = Math.max(prefixLength, value.length - suffixLength)

  const prefix = value.slice(0, prefixLength)
  const changed = value.slice(prefixLength, changedEnd)
  const suffix = value.slice(changedEnd)

  if (prefix) {
    parts.push({ value: prefix, changed: false })
  }
  if (changed) {
    parts.push({ value: changed, changed: true })
  }
  if (suffix) {
    parts.push({ value: suffix, changed: false })
  }

  if (parts.length === 0) {
    parts.push({ value, changed: true })
  }

  return parts
}

function sharedPrefixLength(left: string, right: string) {
  const limit = Math.min(left.length, right.length)
  let index = 0
  while (index < limit && left[index] === right[index]) {
    index += 1
  }
  return index
}

function sharedSuffixLength(left: string, right: string, prefixLength: number) {
  const leftLimit = left.length - prefixLength
  const rightLimit = right.length - prefixLength
  const limit = Math.min(leftLimit, rightLimit)
  let count = 0
  while (
    count < limit &&
    left[left.length - 1 - count] === right[right.length - 1 - count]
  ) {
    count += 1
  }
  return count
}

function serializeBlock(block: SiteDraft['pages'][number]['blocks'][number]) {
  return JSON.stringify({
    type: block.type,
    version: block.version,
    props: block.props,
    settings: block.settings,
    bindings: block.bindings,
  })
}

function summarizeBlock(
  block?: SiteDraft['pages'][number]['blocks'][number],
) {
  if (!block) {
    return ''
  }

  const textValues = collectTextValues(block.props)
    .filter(Boolean)
    .slice(0, 2)

  if (textValues.length > 0) {
    return textValues.join(' · ')
  }

  return block.type.replaceAll('_', ' ')
}

function collectTextValues(value: unknown): string[] {
  if (typeof value === 'string') {
    const normalized = value.trim()
    return normalized ? [normalized] : []
  }
  if (Array.isArray(value)) {
    return value.flatMap((entry) => collectTextValues(entry)).slice(0, 4)
  }
  if (value && typeof value === 'object') {
    const prioritized = ['headline', 'heading', 'title', 'subheadline', 'body', 'intro', 'label']
    const objectValue = value as Record<string, unknown>
    const orderedKeys = [
      ...prioritized.filter((key) => key in objectValue),
      ...Object.keys(objectValue).filter((key) => !prioritized.includes(key)),
    ]
    const texts: string[] = []
    for (const key of orderedKeys) {
      texts.push(...collectTextValues(objectValue[key]))
      if (texts.length >= 4) {
        break
      }
    }
    return texts
  }
  return []
}

function tokenOverlap(left: string, right: string) {
  const leftTokens = new Set(tokenize(left))
  const rightTokens = new Set(tokenize(right))
  return overlapRatio([...leftTokens], [...rightTokens])
}

function overlapRatio(left: string[], right: string[]) {
  if (left.length === 0 && right.length === 0) {
    return 1
  }
  const rightSet = new Set(right)
  const intersection = left.filter((token) => rightSet.has(token)).length
  const union = new Set([...left, ...right]).size
  if (union === 0) {
    return 0
  }
  return intersection / union
}

function tokenize(value: string): string[] {
  return value
    .toLowerCase()
    .split(/[^a-z0-9]+/i)
    .map((token) => token.trim())
    .filter(Boolean)
}

function formatDiffValue(value: unknown): string | undefined {
  if (value === undefined) {
    return undefined
  }
  if (value === null) {
    return 'None'
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
  if (Array.isArray(value)) {
    return value
      .map((entry) => formatDiffValue(entry) || '')
      .filter(Boolean)
      .join(', ')
  }
  return JSON.stringify(value, null, 2)
}

function serializeValue(value: unknown): string {
  if (value === undefined) {
    return ''
  }
  return JSON.stringify(value)
}

function pageDiffKey(
  beforePage: SiteDraft['pages'][number] | undefined,
  afterPage: SiteDraft['pages'][number] | undefined,
  index: number,
) {
  return (
    afterPage?.id ||
    beforePage?.id ||
    afterPage?.slug ||
    beforePage?.slug ||
    `page-${String(index + 1)}`
  )
}

function normalizeLabel(value: string) {
  return value.trim().toLowerCase()
}
