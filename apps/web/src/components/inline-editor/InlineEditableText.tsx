import {
  useEffect,
  useRef,
  type CSSProperties,
  type ClipboardEvent as ReactClipboardEvent,
  type ElementType,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
} from 'react';
import { useInlineEditor } from './context';
import { cn } from '@/lib/utils';

type EditableTag =
  | 'span'
  | 'p'
  | 'h1'
  | 'h2'
  | 'h3'
  | 'h4'
  | 'div'
  | 'blockquote';

export function InlineEditableText({
  blockId,
  path,
  value,
  placeholder,
  multiline = false,
  as = 'span',
  className,
  style,
}: {
  blockId: string;
  path: ReadonlyArray<string | number>;
  value: string;
  placeholder?: string;
  multiline?: boolean;
  as?: EditableTag;
  className?: string;
  style?: CSSProperties;
}) {
  const ctx = useInlineEditor();
  const ref = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (document.activeElement === el) return;
    if (el.textContent !== value) {
      el.textContent = value;
    }
  }, [value]);

  const Tag = as as ElementType;

  if (!ctx?.enabled) {
    return (
      <Tag className={className} style={style}>
        {value || null}
      </Tag>
    );
  }

  function commit() {
    const next = ref.current?.textContent ?? '';
    const normalized = multiline ? next : next.replace(/\s+/g, ' ').trim();
    if (normalized === value) return;
    ctx?.editField(blockId, path, normalized);
  }

  function handleKeyDown(event: ReactKeyboardEvent<HTMLElement>) {
    if (event.key === 'Escape') {
      event.preventDefault();
      if (ref.current) ref.current.textContent = value;
      ref.current?.blur();
      return;
    }
    if (!multiline && event.key === 'Enter') {
      event.preventDefault();
      ref.current?.blur();
      return;
    }
    if (multiline && event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      ref.current?.blur();
    }
  }

  function handleFocus() {
    ctx?.selectBlock(blockId);
  }

  function handleClick(event: ReactMouseEvent<HTMLElement>) {
    event.stopPropagation();
    ctx?.selectBlock(blockId);
  }

  function handlePaste(event: ReactClipboardEvent<HTMLElement>) {
    event.preventDefault();
    const text = event.clipboardData.getData('text/plain');
    const sanitized = multiline ? text : text.replace(/\s+/g, ' ');
    document.execCommand('insertText', false, sanitized);
  }

  return (
    <Tag
      ref={ref as React.Ref<HTMLElement>}
      contentEditable
      suppressContentEditableWarning
      spellCheck
      role="textbox"
      aria-multiline={multiline ? true : undefined}
      aria-label={placeholder}
      data-inline-edit="text"
      data-placeholder={placeholder ?? ''}
      onBlur={commit}
      onFocus={handleFocus}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      onPaste={handlePaste}
      className={cn('inline-edit-target', className)}
      style={style}
    />
  );
}
