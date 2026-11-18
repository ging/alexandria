// Renders a bordered MP4 walkthrough video player with base-path resolution and caption.
// Registers cleanly in MDX so documentation tutorials embed self-hosted screencasts.
import type { ReactNode } from 'react';

interface FlowVideoProps {
  src: string;
  title?: string;
  caption?: ReactNode;
}

const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';

// FlowVideo displays a responsive, dark-framed MP4 walkthrough with captioning.
export function FlowVideo({ src, title, caption }: FlowVideoProps) {
  const normalized = src.startsWith('/') ? src : `/${src}`;
  const resolved = `${basePath}${normalized}`;

  return (
    <figure className="my-6 overflow-hidden rounded-xl border border-fd-border bg-fd-card shadow-sm">
      {title ? (
        <div className="flex items-center justify-between border-b border-fd-border bg-fd-muted/40 px-4 py-2 text-xs font-medium text-fd-muted-foreground">
          <span>{title}</span>
          <a
            href={resolved}
            download
            className="text-fd-primary hover:underline"
          >
            Download MP4
          </a>
        </div>
      ) : null}
      <video
        src={resolved}
        controls
        playsInline
        preload="metadata"
        className="aspect-[16/10] w-full bg-slate-950"
      />
      {caption ? (
        <figcaption className="border-t border-fd-border px-4 py-2.5 text-xs text-fd-muted-foreground">
          {caption}
        </figcaption>
      ) : null}
    </figure>
  );
}
