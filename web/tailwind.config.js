/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: ["class", '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        // IBM Carbon monochrome palette
        carbon: {
          background: "#161616",   // deepest bg
          surface:    "#262626",   // card / sidebar surface
          surface2:   "#393939",   // elevated surface / border
          surface3:   "#525252",   // subtle accent / selected
          text:       "#f4f4f4",   // primary text
          textSub:    "#c6c6c6",   // secondary text
          textMuted:  "#8d8d8d",   // muted / placeholder
          border:     "#393939",   // border
          hover:      "#353535",   // hover state
        },
      },
      borderRadius: {
        card: "0.75rem",
      },
    },
  },
  plugins: [],
};
