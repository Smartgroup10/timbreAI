"use client";

// Logo asset is served from /public/brand/ — we use <img> directly instead of next/image to keep
// the bundle small (these are tiny SVGs that benefit nothing from optimization).

export function BrandMark({ size = 36, className = "" }: { size?: number; className?: string }) {
  return (
    <img
      src="/brand/mark-primary.svg"
      alt="timbre.ai"
      width={size}
      height={size}
      className={className}
      style={{ display: "block" }}
    />
  );
}

export function BrandLockup({ height = 40 }: { height?: number }) {
  return (
    <img
      src="/brand/lockup-horizontal-ai.svg"
      alt="timbre.ai"
      height={height}
      style={{ display: "block", height }}
    />
  );
}
