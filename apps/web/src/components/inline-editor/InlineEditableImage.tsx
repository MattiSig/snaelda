import { useRef, useState, type ReactNode } from 'react';
import { ImagePlus, Upload, X } from 'lucide-react';
import { useInlineEditor } from './context';
import { ImagePickerModal } from './ImagePickerModal';
import {
  completeAssetUpload,
  createAssetUploadURL,
  type ImageApplyResponse,
} from '@/lib/api';
import { readImageDimensions } from '@/lib/assets';
import { cn } from '@/lib/utils';

type ImageRef = { assetId: string; alt: string };

export function InlineEditableImage({
  blockId,
  path,
  image,
  emptyLabel = 'Add image',
  className,
  overlayPosition = 'corner',
  rounded = true,
  children,
}: {
  blockId: string;
  path: ReadonlyArray<string | number>;
  image: ImageRef | null;
  emptyLabel?: string;
  className?: string;
  overlayPosition?: 'corner' | 'center';
  rounded?: boolean;
  children: ReactNode;
}) {
  const ctx = useInlineEditor();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState('');

  if (!ctx?.enabled) {
    return <>{children}</>;
  }

  async function handleUpload(file: File) {
    if (!ctx) return;
    setUploading(true);
    setUploadError('');
    try {
      const ticket = await createAssetUploadURL({
        siteId: ctx.siteId,
        fileName: file.name,
        contentType: file.type,
        sizeBytes: file.size,
      });
      const headers = new Headers(ticket.upload.headers ?? {});
      if (!headers.has('Content-Type') && file.type) {
        headers.set('Content-Type', file.type);
      }
      const upload = await fetch(ticket.upload.url, {
        method: ticket.upload.method || 'PUT',
        headers,
        body: file,
      });
      if (!upload.ok) {
        throw new Error(`Upload failed (${upload.status})`);
      }
      const dimensions = await readImageDimensions(file).catch(() => null);
      const completed = await completeAssetUpload(ticket.asset.id, {
        width: dimensions?.width,
        height: dimensions?.height,
      });
      ctx.editField(blockId, path, {
        assetId: completed.asset.id,
        alt: completed.asset.altText || image?.alt || '',
      });
    } catch (error) {
      setUploadError(
        error instanceof Error ? error.message : 'Could not upload image',
      );
    } finally {
      setUploading(false);
    }
  }

  function handleApplied(response: ImageApplyResponse) {
    if (response.image) {
      ctx?.editField(blockId, path, {
        assetId: response.image.assetId,
        alt: response.image.alt,
      });
    }
    ctx?.onImageApplied?.(response);
    setPickerOpen(false);
  }

  function handleClear(event: React.MouseEvent) {
    event.stopPropagation();
    ctx?.editField(blockId, path, undefined);
  }

  function handleOpenPicker(event: React.MouseEvent) {
    event.stopPropagation();
    ctx?.selectBlock(blockId);
    setPickerOpen(true);
  }

  function handleUploadClick(event: React.MouseEvent) {
    event.stopPropagation();
    ctx?.selectBlock(blockId);
    fileInputRef.current?.click();
  }

  const hasImage = image !== null;

  return (
    <div
      className={cn(
        'group/image relative isolate inline-block w-full',
        rounded && 'rounded-[var(--radius-inner)]',
        className,
      )}
      onClick={(event) => {
        event.stopPropagation();
        ctx?.selectBlock(blockId);
      }}
    >
      {children}

      {/* Hover affordance */}
      <div
        aria-hidden="true"
        className={cn(
          'pointer-events-none absolute inset-0 z-10 transition-opacity duration-200',
          rounded && 'rounded-[var(--radius-inner)]',
          'bg-[oklch(8%_0.02_336_/_0)] group-hover/image:bg-[oklch(8%_0.02_336_/_0.42)]',
          'ring-0 group-hover/image:ring-2 group-hover/image:ring-[var(--thread-violet)]/70',
          'ring-inset',
          'opacity-0 group-hover/image:opacity-100',
        )}
      />

      {/* Action buttons */}
      <div
        className={cn(
          'pointer-events-none absolute z-20 flex flex-wrap items-center gap-2 transition-opacity duration-200',
          'opacity-0 group-hover/image:opacity-100',
          overlayPosition === 'corner'
            ? 'right-3 top-3'
            : 'left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2',
        )}
      >
        <button
          type="button"
          onClick={handleOpenPicker}
          disabled={uploading}
          className={cn(
            'pointer-events-auto inline-flex items-center gap-1.5 rounded-full',
            'border border-[oklch(98%_0.005_336_/_0.18)] bg-[oklch(15%_0.02_336_/_0.92)] px-3 py-1.5',
            'text-xs font-semibold text-[#F9F7F2] shadow-[0_8px_22px_oklch(8%_0.02_336_/_0.5)] backdrop-blur',
            'transition-transform hover:-translate-y-px hover:bg-[oklch(20%_0.025_336_/_0.95)]',
            'disabled:cursor-not-allowed disabled:opacity-60',
          )}
        >
          <ImagePlus className="size-3.5" aria-hidden />
          {hasImage ? 'Find a better one' : emptyLabel}
        </button>
        <button
          type="button"
          onClick={handleUploadClick}
          disabled={uploading}
          className={cn(
            'pointer-events-auto inline-flex items-center gap-1.5 rounded-full',
            'border border-[oklch(98%_0.005_336_/_0.18)] bg-[oklch(15%_0.02_336_/_0.78)] px-3 py-1.5',
            'text-xs font-semibold text-[#F9F7F2] shadow-[0_8px_22px_oklch(8%_0.02_336_/_0.5)] backdrop-blur',
            'transition-transform hover:-translate-y-px hover:bg-[oklch(20%_0.025_336_/_0.95)]',
            'disabled:cursor-not-allowed disabled:opacity-60',
          )}
        >
          <Upload className="size-3.5" aria-hidden />
          {uploading ? 'Uploading…' : 'Upload'}
        </button>
        {hasImage ? (
          <button
            type="button"
            onClick={handleClear}
            disabled={uploading}
            aria-label="Remove image"
            className={cn(
              'pointer-events-auto inline-flex size-7 items-center justify-center rounded-full',
              'border border-[oklch(98%_0.005_336_/_0.18)] bg-[oklch(15%_0.02_336_/_0.78)]',
              'text-[#F9F7F2] shadow-[0_8px_22px_oklch(8%_0.02_336_/_0.5)] backdrop-blur',
              'transition-transform hover:-translate-y-px hover:bg-[oklch(20%_0.025_336_/_0.95)]',
              'disabled:cursor-not-allowed disabled:opacity-60',
            )}
          >
            <X className="size-3.5" aria-hidden />
          </button>
        ) : null}
      </div>

      {uploadError ? (
        <div className="pointer-events-none absolute bottom-3 left-3 z-20 rounded-md bg-[var(--destructive)] px-2 py-1 text-xs font-bold text-white shadow">
          {uploadError}
        </div>
      ) : null}

      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={(event) => {
          const file = event.target.files?.[0];
          if (file) {
            void handleUpload(file);
          }
          event.target.value = '';
        }}
      />

      {pickerOpen ? (
        <ImagePickerModal
          context={{
            siteId: ctx.siteId,
            blockId,
            path: path.map(String),
            currentAlt: image?.alt ?? '',
          }}
          onClose={() => setPickerOpen(false)}
          onApplied={handleApplied}
        />
      ) : null}
    </div>
  );
}
