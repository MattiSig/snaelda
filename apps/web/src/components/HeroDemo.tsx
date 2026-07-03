import { Pause, Play } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { cn } from '@/lib/utils'

type VideoSource = {
  src: string
  type: 'video/webm' | 'video/mp4'
  media?: string
}

interface HeroDemoProps {
  id?: string
  posterSrc: string
  sources: VideoSource[]
  eyebrow?: string
  caption?: string
  className?: string
}

function useReducedMotion() {
  const [reduced, setReduced] = useState(false)
  useEffect(() => {
    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    const update = () => setReduced(query.matches)
    update()
    query.addEventListener('change', update)
    return () => query.removeEventListener('change', update)
  }, [])
  return reduced
}

export function HeroDemo({
  id = 'hero-demo',
  posterSrc,
  sources,
  eyebrow,
  caption,
  className,
}: HeroDemoProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const reducedMotion = useReducedMotion()
  const [isPlaying, setIsPlaying] = useState(false)
  const userPausedRef = useRef(false)

  useEffect(() => {
    const video = videoRef.current
    if (!video) {
      return
    }
    if (reducedMotion) {
      video.pause()
      return
    }
    if (!userPausedRef.current) {
      video.play().catch(() => {
        // Autoplay may be blocked; the poster stays and the user can tap play.
      })
    }
  }, [reducedMotion])

  useEffect(() => {
    const video = videoRef.current
    if (!video) {
      return
    }
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry) {
          return
        }
        if (entry.isIntersecting) {
          if (!userPausedRef.current && !reducedMotion) {
            video.play().catch(() => {})
          }
        } else {
          video.pause()
        }
      },
      { threshold: 0.25 },
    )
    observer.observe(video)
    return () => observer.disconnect()
  }, [reducedMotion])

  function togglePlay() {
    const video = videoRef.current
    if (!video) {
      return
    }
    if (video.paused) {
      userPausedRef.current = false
      video.play().catch(() => {})
    } else {
      userPausedRef.current = true
      video.pause()
    }
  }

  const label = isPlaying ? 'Pause demo' : 'Play demo'

  return (
    <figure
      id={id}
      className={cn('mx-auto w-full max-w-[1100px]', className)}
      aria-label="Snaelda demo: describe what you do, see a real first draft, refine, and publish."
    >
      {eyebrow ? (
        <p className="mb-4 text-center text-xs font-bold uppercase tracking-[0.18em] text-[color-mix(in_oklch,var(--thread-mauve)_72%,var(--paper))]">
          {eyebrow}
        </p>
      ) : null}

      <div
        className="relative aspect-[16/10] w-full overflow-hidden rounded-[18px] border border-[color-mix(in_oklch,var(--thread-mauve)_36%,var(--border))] bg-[var(--surface-1)] shadow-[0_28px_80px_-28px_oklch(16%_0.05_336_/_0.7)]"
        style={{
          boxShadow:
            'inset 0 0 0 1px color-mix(in oklch, var(--thread-mauve) 14%, transparent), 0 28px 80px -28px oklch(16% 0.05 336 / 0.7)',
        }}
      >
        <video
          ref={videoRef}
          className="block size-full object-cover"
          autoPlay={!reducedMotion}
          muted
          loop
          playsInline
          preload="metadata"
          poster={posterSrc}
          onPlay={() => setIsPlaying(true)}
          onPause={() => setIsPlaying(false)}
        >
          {sources.map((source) => (
            <source
              key={`${source.src}-${source.type}-${source.media ?? 'all'}`}
              src={source.src}
              type={source.type}
              media={source.media}
            />
          ))}
        </video>

        <button
          type="button"
          onClick={togglePlay}
          aria-label={label}
          className="absolute bottom-3 right-3 inline-flex size-11 items-center justify-center rounded-full border border-[color-mix(in_oklch,var(--thread-teal)_42%,var(--border))] bg-[color-mix(in_oklch,var(--surface-0)_68%,transparent)] text-[var(--paper)] backdrop-blur-sm transition-[background,border-color,color,transform] hover:-translate-y-px hover:border-[var(--thread-teal)] hover:bg-[var(--surface-2)] focus-visible:border-[var(--thread-teal)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--thread-teal)]"
        >
          {isPlaying ? (
            <Pause aria-hidden className="size-4" />
          ) : (
            <Play aria-hidden className="size-4" />
          )}
        </button>
      </div>

      {caption ? (
        <figcaption className="mx-auto mt-5 max-w-2xl text-center text-sm leading-6 text-[color-mix(in_oklch,var(--paper-muted)_82%,transparent)]">
          {caption}
        </figcaption>
      ) : null}
    </figure>
  )
}
