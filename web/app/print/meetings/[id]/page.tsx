"use client";

import { Suspense, use } from "react";
import { MeetingPrint } from "@/components/kairso/MeetingPrint";

/** Bản in biên bản họp (A4) — mở từ nút "Xuất PDF", `?auto=1` tự bật hộp thoại in/lưu PDF. */
export default function PrintMeetingPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return (
    <Suspense>
      <MeetingPrint id={id} />
    </Suspense>
  );
}
