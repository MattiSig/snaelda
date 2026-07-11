import { getAPIBaseURL, type AssetRecord } from './api'

export function buildDraftAssetURL(assetId: string) {
  return new URL(`/api/assets/${assetId}/content`, getAPIBaseURL()).toString()
}

export function buildPreviewAssetURL(previewToken: string, assetId: string) {
  return new URL(
    `/api/public/preview/${previewToken}/assets/${assetId}`,
    getAPIBaseURL(),
  ).toString()
}

export function buildPublishedAssetURL(siteSlug: string, assetId: string) {
  return new URL(
    `/api/public/sites/${siteSlug}/assets/${assetId}`,
    getAPIBaseURL(),
  ).toString()
}

export async function readImageDimensions(file: File) {
  const objectURL = URL.createObjectURL(file)

  try {
    const dimensions = await new Promise<{ width: number; height: number }>(
      (resolve, reject) => {
        const image = new Image()
        image.onload = () => {
          resolve({
            width: image.naturalWidth,
            height: image.naturalHeight,
          })
        }
        image.onerror = () =>
          reject(new Error('Could not read image dimensions'))
        image.src = objectURL
      },
    )

    return dimensions
  } finally {
    URL.revokeObjectURL(objectURL)
  }
}

export function formatAssetFileSize(sizeBytes?: number) {
  if (!sizeBytes || sizeBytes <= 0) {
    return 'Unknown size'
  }

  if (sizeBytes < 1024) {
    return `${sizeBytes} B`
  }

  if (sizeBytes < 1024 * 1024) {
    return `${(sizeBytes / 1024).toFixed(1)} KB`
  }

  return `${(sizeBytes / (1024 * 1024)).toFixed(1)} MB`
}

export function describeAssetDimensions(asset: AssetRecord) {
  const { width, height } = asset.metadata
  if (!width || !height) {
    return 'Dimensions pending'
  }
  return `${width} x ${height}`
}
