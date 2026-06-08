import type { ReactNode } from "react";
import { cookies, headers } from "next/headers";
import "./globals.css";
import "flag-icons/css/flag-icons.min.css";
import { I18nProvider } from "@/components/i18n/I18nProvider";
import { ThemeProvider } from "@/components/theme/ThemeProvider";
import { ControlsBar } from "@/components/ControlsBar";
import { resolveTheme, THEME_COOKIE } from "@/lib/theme";
import { pickLanguage, COOKIE as LANG_COOKIE } from "@/lib/i18n/detect";
import { SUPPORTED, DEFAULT_LANGUAGE, isRtl } from "@/lib/i18n/locales";

export const metadata = {
  title: "BombVault",
  description: "Backup & disaster recovery for Docker containers and KVM/libvirt VMs.",
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  // ── Theme: read cookie server-side → no flash ─────────────────────────
  const cookieStore = await cookies();
  const themeCookie = cookieStore.get(THEME_COOKIE)?.value;
  const theme = resolveTheme(themeCookie);

  // ── Language: cookie wins, then Accept-Language, then fallback ─────────
  const headerStore = await headers();
  const langCookie = cookieStore.get(LANG_COOKIE)?.value;
  const acceptLanguage = headerStore.get("accept-language");
  const lang = pickLanguage(langCookie, acceptLanguage, SUPPORTED, DEFAULT_LANGUAGE);
  const dir = isRtl(lang) ? "rtl" : "ltr";

  // Blocking inline script: sets data-theme before first paint so first-time
  // visitors (no cookie) get the correct OS-preference theme without a flash.
  // This runs synchronously in <head>, before any CSS is applied.
  const noFlashScript = `(function(){try{var c=document.cookie.split(';').map(function(s){return s.trim();}),t=c.find(function(s){return s.startsWith('${THEME_COOKIE}=');});if(t){document.documentElement.dataset.theme=t.split('=')[1];}else{document.documentElement.dataset.theme=window.matchMedia('(prefers-color-scheme: light)').matches?'light':'dark';}}catch(e){}})();`;

  return (
    <html lang={lang} dir={dir} data-theme={theme} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: noFlashScript }} />
      </head>
      <body>
        <ThemeProvider initialTheme={theme}>
          <I18nProvider initialLanguage={lang}>
            <ControlsBar />
            {children}
          </I18nProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
