"use client";

import { Suspense } from "react";
import { OAuthConsent } from "@/components/settings/OAuthConsent";

export default function OAuthAuthorizePage() {
  return (
    <Suspense>
      <OAuthConsent />
    </Suspense>
  );
}
