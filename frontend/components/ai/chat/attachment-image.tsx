"use client";

import { useEffect, useState } from "react";
import { FileTextIcon } from "@radix-ui/react-icons";
import { fetchAttachmentBlob, type AttachmentRef } from "@/lib/ai-chat";
import { cn } from "@/lib/utils";

export function AttachmentImage({ attachment, className, compact = false }: {
  attachment: AttachmentRef;
  className?: string;
  compact?: boolean;
}) {
  const [url, setUrl] = useState("");
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    let objectURL = "";
    fetchAttachmentBlob(attachment.id).then((blob) => {
      if (!active) return;
      objectURL = URL.createObjectURL(blob);
      setUrl(objectURL);
    }).catch(() => {
      if (active) setFailed(true);
    });
    return () => {
      active = false;
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [attachment.id]);

  if (failed) {
    return (
      <span className={cn("inline-flex items-center gap-1.5 rounded-md border border-border bg-neutral-2 px-2 py-1 text-label-12 text-neutral-7", className)}>
        <FileTextIcon className="size-3.5" />{attachment.name}
      </span>
    );
  }
  if (!url) {
    return <span className={cn("block animate-pulse rounded-md bg-neutral-3", compact ? "size-14" : "h-40 w-56", className)} aria-label="图片加载中" />;
  }
  return (
    <a href={url} target="_blank" rel="noreferrer" title={attachment.name} className={cn("block overflow-hidden rounded-md border border-border bg-neutral-2 focus-visible:ring-2 focus-visible:ring-accent/40", className)}>
              {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={url} alt={attachment.name} className={cn("object-cover", compact ? "size-14" : "max-h-56 w-auto")} loading="lazy" />
    </a>
  );
}