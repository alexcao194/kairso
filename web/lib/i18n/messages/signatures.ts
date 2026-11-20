import { defineMessages } from "../define";

export const signatures = defineMessages({
  vi: {
    docSign: {
      title: "Chữ ký",
      printHint: "(Ký, ghi rõ họ tên)",
      preparer: "Người lập",
      controller: "Người kiểm soát",
      unsigned: "Chưa ký",
      sign: "Ký",
      unsign: "Gỡ chữ ký",
      signedOk: "Đã ký",
      unsignedOk: "Đã gỡ chữ ký",
      unsignConfirm: "Gỡ chữ ký này?",
      unsignDesc: "Văn bản sẽ không còn dấu tên và ngày ở vai trò này. Bạn có thể ký lại sau.",
      signedOn: (date: string) => `Ký ngày ${date}`,
      quoteHint: "Dấu tên và ngày của người lập sẽ in ở cuối bản PDF.",
      onlyRecorder: "Chỉ người ghi chép (người tạo hoặc người trong mục ghi chép) mới ký được.",
      onlyHolder: "Chỉ chủ workspace mới ký được.",
      onlyManager: "Chỉ quản lý workspace mới ký được.",
    },
  },
  en: {
    docSign: {
      title: "Signatures",
      printHint: "(Signature & full name)",
      preparer: "Prepared by",
      controller: "Reviewed by",
      unsigned: "Not signed",
      sign: "Sign",
      unsign: "Remove signature",
      signedOk: "Signed",
      unsignedOk: "Signature removed",
      unsignConfirm: "Remove this signature?",
      unsignDesc: "The document will lose the name and date stamp for this role. You can sign again later.",
      signedOn: (date: string) => `Signed on ${date}`,
      quoteHint: "The preparer's name and date stamp print at the end of the PDF.",
      onlyRecorder: "Only the note-taker (the creator or a listed recorder) can sign.",
      onlyHolder: "Only the workspace holder can sign.",
      onlyManager: "Only workspace managers can sign.",
    },
  },
});
