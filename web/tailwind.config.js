/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: ["class", '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        // IBM Carbon palette — values are CSS custom properties so that
        // toggling data-theme on <html> instantly re-colours the whole app.
        carbon: {
          background: "var(--carbon-bg)",
          surface:    "var(--carbon-surface)",
          surface2:   "var(--carbon-surface2)",
          surface3:   "var(--carbon-surface3)",
          text:       "var(--carbon-text)",
          textSub:    "var(--carbon-text-sub)",
          textMuted:  "var(--carbon-text-muted)",
          border:     "var(--carbon-border)",
          hover:      "var(--carbon-hover)",
        },
      },
      borderRadius: {
        card: "0.75rem",
      },
    },
  },
  plugins: [],
};
