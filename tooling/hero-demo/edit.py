#!/usr/bin/env python3
"""Edit the Playwright-recorded master into the shipping hero-demo assets.

Pipeline:
  1. Read beats.json from the capture step.
  2. Build a filtergraph that:
     - trims into three segments around the generation wait,
     - speed-ramps ONLY the generation-wait segment via setpts,
     - burns in the four short captions at the right times,
     - tail-crossfades the last 0.3s back onto the first 0.3s for a seamless loop.
  3. Derive hero-1280.{webm,mp4} and hero-768.{webm,mp4}, audio stripped.
  4. Grab the first frame as poster.webp and poster.jpg.
  5. Assert each web variant is under the 2.5 MB ceiling.

Usage:
  python3 tooling/hero-demo/edit.py \
    --in apps/web/public/media/hero-demo/_raw \
    --out apps/web/public/media/hero-demo \
    --target-duration 18

If --target-duration is set, the script picks the gen-wait speed ramp so the
final clip is close to that length (default 18s, must stay in 15-22s).

Requires: ffmpeg in PATH, ffprobe in PATH, Python 3.10+.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

MAX_BYTES = int(2.5 * 1024 * 1024)
HARD_CEILING_BYTES = 4 * 1024 * 1024


@dataclass
class Beats:
    landing_loaded: float
    spin_clicked: float
    site_rendered: float
    capture_end: float

    @classmethod
    def from_json(cls, path: Path) -> "Beats":
        data = json.loads(path.read_text())
        lookup = {b["name"]: b["t"] for b in data["beats"]}
        required = ["landing.loaded", "spin.clicked", "site.rendered", "capture.end"]
        missing = [r for r in required if r not in lookup]
        if missing:
            raise SystemExit(f"beats.json missing required beats: {missing}")
        return cls(
            landing_loaded=lookup["landing.loaded"],
            spin_clicked=lookup["spin.clicked"],
            site_rendered=lookup["site.rendered"],
            capture_end=lookup["capture.end"],
        )


def run(cmd: list[str]) -> None:
    print(f"$ {' '.join(cmd)}")
    subprocess.run(cmd, check=True)


def probe_duration(path: Path) -> float:
    result = subprocess.run(
        [
            "ffprobe",
            "-v",
            "error",
            "-show_entries",
            "format=duration",
            "-of",
            "default=noprint_wrappers=1:nokey=1",
            str(path),
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    return float(result.stdout.strip())


def build_filter(beats: Beats, target_duration: float) -> tuple[str, float]:
    """Build the filtergraph that trims, ramps, captions, and loops the master.

    Returns (filter_complex, predicted_output_duration).
    """
    pre_start = beats.landing_loaded
    pre_end = beats.spin_clicked + 0.4
    wait_start = pre_end
    wait_end = beats.site_rendered
    post_start = wait_end
    post_end = beats.capture_end

    pre_dur = max(pre_end - pre_start, 0.5)
    wait_dur = max(wait_end - wait_start, 0.5)
    post_dur = max(post_end - post_start, 0.5)

    # Hold the rest at real time; only ramp the gen wait.
    # Solve setpts factor so pre + wait_ramped + post == target_duration.
    remaining = target_duration - pre_dur - post_dur
    if remaining <= 0.5:
        # Hit the floor; cap the ramp at 8x and let total be slightly long.
        ramp = 1.0 / 8.0
    else:
        ramp = remaining / wait_dur
        ramp = max(min(ramp, 1.0), 1.0 / 12.0)

    predicted = pre_dur + wait_dur * ramp + post_dur

    # Captions: lower-left, BVP-700-uppercase via fontcolor + box-tint.
    cap = (
        "drawtext=fontcolor=0xe4e2dd:fontsize=24:x=48:y=h-96:"
        "box=1:boxcolor=0x131411A0:boxborderw=12:"
        "fontfile=/usr/share/fonts/TTF/BeVietnamPro-Bold.ttf"
    )
    cap_pre = f"{cap}:text='DESCRIBE WHAT YOU DO':enable='between(t,0.4,{pre_dur:.2f})'"
    cap_spin = f"{cap}:text='SPIN':enable='between(t,{max(pre_dur - 1.0, 0):.2f},{pre_dur:.2f})'"
    cap_draft = (
        f"{cap}:text='A REAL FIRST DRAFT':"
        f"enable='between(t,{pre_dur + 0.4:.2f},{pre_dur + 4.0:.2f})'"
    )
    cap_refine = (
        f"{cap}:text='REFINE ANYTHING':"
        f"enable='between(t,{pre_dur + 4.0:.2f},{pre_dur + 7.0:.2f})'"
    )
    cap_publish = (
        f"{cap}:text='PUBLISH':"
        f"enable='between(t,{predicted - 2.0:.2f},{predicted:.2f})'"
    )

    # NOTE: ramp the wait, then concat. Trim+setpts in three branches.
    filter_complex = (
        f"[0:v]trim=start={pre_start}:end={pre_end},setpts=PTS-STARTPTS[pre];"
        f"[0:v]trim=start={wait_start}:end={wait_end},setpts=({ramp:.4f})*(PTS-STARTPTS)[wait];"
        f"[0:v]trim=start={post_start}:end={post_end},setpts=PTS-STARTPTS[post];"
        "[pre][wait][post]concat=n=3:v=1:a=0[edited];"
        f"[edited]{cap_pre},{cap_spin},{cap_draft},{cap_refine},{cap_publish}[captioned];"
        # 0.3s tail crossfade so the loop seam reads smooth.
        f"[captioned]split=2[main][tail];"
        f"[tail]trim=start={predicted - 0.3:.3f}:end={predicted:.3f},setpts=PTS-STARTPTS,format=yuva420p,fade=t=out:st=0:d=0.3:alpha=1[fadetail];"
        f"[main][fadetail]overlay=eof_action=pass:enable='gte(t,{predicted - 0.3:.3f})'[v]"
    )
    return filter_complex, predicted


def encode_variant(
    master: Path,
    out: Path,
    filter_complex: str,
    width: int,
    codec: str,
    crf: int,
) -> None:
    scale_chain = f"[v]scale={width}:-2[vout]"
    full_filter = f"{filter_complex};{scale_chain}"
    if codec == "h264":
        cmd = [
            "ffmpeg",
            "-y",
            "-i",
            str(master),
            "-filter_complex",
            full_filter,
            "-map",
            "[vout]",
            "-an",
            "-c:v",
            "libx264",
            "-profile:v",
            "high",
            "-crf",
            str(crf),
            "-preset",
            "slow",
            "-movflags",
            "+faststart",
            str(out),
        ]
    elif codec == "vp9":
        cmd = [
            "ffmpeg",
            "-y",
            "-i",
            str(master),
            "-filter_complex",
            full_filter,
            "-map",
            "[vout]",
            "-an",
            "-c:v",
            "libvpx-vp9",
            "-crf",
            str(crf),
            "-b:v",
            "0",
            "-row-mt",
            "1",
            str(out),
        ]
    else:
        raise ValueError(f"unknown codec: {codec}")
    run(cmd)


def adaptive_encode(
    master: Path,
    out: Path,
    filter_complex: str,
    width: int,
    codec: str,
    start_crf: int,
) -> Path:
    crf = start_crf
    while crf <= start_crf + 8:
        encode_variant(master, out, filter_complex, width, codec, crf)
        size = out.stat().st_size
        print(f"  -> {out.name} {size / 1024:.1f} KB @ crf={crf}")
        if size <= MAX_BYTES:
            return out
        crf += 2
    if out.stat().st_size > HARD_CEILING_BYTES:
        raise SystemExit(
            f"{out.name} exceeded hard ceiling of {HARD_CEILING_BYTES / 1024 / 1024:.1f} MB"
        )
    print(
        f"WARNING: {out.name} above 2.5 MB target but below 4 MB ceiling; shipping anyway"
    )
    return out


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--in", dest="in_dir", default="apps/web/public/media/hero-demo/_raw")
    parser.add_argument("--out", default="apps/web/public/media/hero-demo")
    parser.add_argument("--target-duration", type=float, default=18.0)
    args = parser.parse_args()

    in_dir = Path(args.in_dir)
    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    master = in_dir / "master.webm"
    beats_path = in_dir / "beats.json"
    if not master.exists():
        raise SystemExit(f"master video not found: {master}")
    if not beats_path.exists():
        raise SystemExit(f"beats.json not found: {beats_path}")

    beats = Beats.from_json(beats_path)
    raw_duration = probe_duration(master)
    print(
        f"raw master duration: {raw_duration:.2f}s, "
        f"target output: {args.target_duration:.1f}s"
    )

    filter_complex, predicted = build_filter(beats, args.target_duration)
    print(f"predicted output duration: {predicted:.2f}s")
    if not (15.0 <= predicted <= 22.0):
        print(
            f"WARNING: predicted duration {predicted:.2f}s is outside the 15-22s "
            f"spec window. Adjust --target-duration or re-capture."
        )

    adaptive_encode(master, out_dir / "hero-1280.mp4", filter_complex, 1280, "h264", 24)
    adaptive_encode(master, out_dir / "hero-1280.webm", filter_complex, 1280, "vp9", 33)
    adaptive_encode(master, out_dir / "hero-768.mp4", filter_complex, 768, "h264", 25)
    adaptive_encode(master, out_dir / "hero-768.webm", filter_complex, 768, "vp9", 34)

    poster_master = in_dir / "poster.png"
    if poster_master.exists():
        run([
            "ffmpeg", "-y", "-i", str(poster_master),
            "-vf", "scale=1280:-2",
            "-q:v", "2", str(out_dir / "poster.jpg"),
        ])
        run([
            "ffmpeg", "-y", "-i", str(poster_master),
            "-vf", "scale=1280:-2",
            "-c:v", "libwebp", "-quality", "88", str(out_dir / "poster.webp"),
        ])
    else:
        print("poster.png not found in _raw — extracting from the encoded mp4")
        run([
            "ffmpeg", "-y", "-i", str(out_dir / "hero-1280.mp4"),
            "-vframes", "1", "-q:v", "2", str(out_dir / "poster.jpg"),
        ])
        run([
            "ffmpeg", "-y", "-i", str(out_dir / "hero-1280.mp4"),
            "-vframes", "1", "-c:v", "libwebp", "-quality", "88",
            str(out_dir / "poster.webp"),
        ])

    print("\nDone. Outputs:")
    for f in sorted(out_dir.glob("*")):
        if f.is_file():
            print(f"  {f.name}: {f.stat().st_size / 1024:.1f} KB")
    return 0


if __name__ == "__main__":
    sys.exit(main())
