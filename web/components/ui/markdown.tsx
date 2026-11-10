import type { ComponentProps } from "react";
import rehypeSanitize from "rehype-sanitize";
import { MENTION_HREF_PREFIX } from "@/lib/domain/mention";

/**
 * Bộ preview của `@uiw/react-md-editor` chạy `rehype-raw` (cho phép HTML thô) mà không sanitize.
 * Ghi chú/mô tả là nội dung nhiều người cùng xem nên bắt buộc lọc bằng schema mặc định của GitHub
 * (bỏ <script>, on*=, javascript: URL…). Plugin này được nối SAU rehype-raw của thư viện.
 */
export const MD_REHYPE_PLUGINS = [rehypeSanitize];

/**
 * Link ra ngoài mở tab mới và không lộ `window.opener`; link neo (#…) giữ nguyên.
 * `#mention-<userId>` (xem `mentionsToMarkdown`) hiện thành chip nhắc tên, không phải link.
 */
export const MD_COMPONENTS = {
  a: ({ node: _node, ...props }: ComponentProps<"a"> & { node?: unknown }) =>
    props.href?.startsWith(MENTION_HREF_PREFIX) ? (
      <span className="rounded bg-accent-soft px-1 font-medium text-accent-ink">{props.children}</span>
    ) : props.href?.startsWith("#") ? (
      <a {...props} />
    ) : (
      <a {...props} target="_blank" rel="noopener noreferrer" />
    ),
};
