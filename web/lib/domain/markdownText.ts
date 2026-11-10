/**
 * Markdown → một dòng chữ thuần cho chỗ xem trước (danh sách thảo luận, thông báo…):
 * ảnh thành "[Ảnh]", link giữ chữ, bỏ ký hiệu định dạng.
 */
export function markdownToPlain(md: string): string {
  return md
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/!\[[^\]]*\]\([^)]*\)/g, "[Ảnh]")
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/^\s{0,3}(#{1,6}\s+|>\s?|[-*+]\s+(\[[ xX]\]\s+)?|\d+\.\s+)/gm, "")
    .replace(/(\*\*|__|~~|`)/g, "")
    .replace(/\s+/g, " ")
    .trim();
}
