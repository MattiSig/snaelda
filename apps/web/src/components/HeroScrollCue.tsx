import { ChevronDown } from 'lucide-react'
import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'

interface HeroScrollCueProps {
  targetId: string
  label?: string
  className?: string
}

export function HeroScrollCue({
  targetId,
  label = 'See it spin',
  className,
}: HeroScrollCueProps) {
  const [visible, setVisible] = useState(true)

  useEffect(() => {
    const update = () => {
      const threshold = Math.max(window.innerHeight * 0.25, 120)
      setVisible(window.scrollY < threshold)
    }
    update()
    window.addEventListener('scroll', update, { passive: true })
    window.addEventListener('resize', update)
    return () => {
      window.removeEventListener('scroll', update)
      window.removeEventListener('resize', update)
    }
  }, [])

  function handleClick() {
    const target = document.getElementById(targetId)
    if (!target) {
      return
    }
    const prefersReducedMotion = window.matchMedia(
      '(prefers-reduced-motion: reduce)',
    ).matches
    target.scrollIntoView({
      behavior: prefersReducedMotion ? 'auto' : 'smooth',
      block: 'start',
    })
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label={`${label}: scroll to demo`}
      aria-hidden={!visible}
      tabIndex={visible ? 0 : -1}
      className={cn(
        'group inline-flex flex-col items-center gap-2 rounded-md px-3 py-2 text-[color-mix(in_oklch,var(--paper-muted)_82%,transparent)] outline-none transition-[opacity,color,transform] duration-500 hover:text-[var(--thread-mauve)] focus-visible:text-[var(--thread-mauve)] focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-[var(--thread-teal)]',
        visible
          ? 'opacity-100'
          : 'pointer-events-none -translate-y-1 opacity-0',
        className,
      )}
    >
      <span className="text-[10px] font-bold uppercase tracking-[0.22em]">
        {label}
      </span>
      <ChevronDown
        aria-hidden
        className="size-5 animate-[heroCueBob_3.4s_ease-in-out_infinite] motion-reduce:animate-none"
      />
    </button>
  )
}
