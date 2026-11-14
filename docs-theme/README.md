# docs-theme

A shared look for the MkDocs Material sites of the apps: GlimStone's Carbon greys in dark and light, Bree Serif headings, the app's accent on the current page and the main button, admonitions as cards with a title badge, and a landing hero with the logo pressed into a relief ground, like the store images.

The folder is the same, byte for byte, in every repo that uses it. The master is `config/templates/docs-theme/` in the bootstrapping repo: change it there, then copy the whole folder into each app again. An app adopts it by copying `docs-theme/` into its root, pointing `custom_dir` at it and filling in `extra.docs_theme`, as below.

Everything app-specific stays in the app's `mkdocs.yml`. The theme reads:

| Setting | Where | What it does |
|---|---|---|
| Name | `site_name` | Header and hero |
| Logo | `theme.logo` | Header, hero and the relief behind it |
| Navigation | `nav` | The rail |
| Accent | `extra.docs_theme.accent` | Active page, primary button, chosen tab, links. Defaults to `#FCC419` |
| Ink on the accent | `extra.docs_theme.accent_contrast` | Defaults to `#161616` |
| Flags | `extra.docs_theme.flags` | `true` shows flags in the language menu (the site has to load flag-icons). Otherwise the menu shows language codes |
| Relief | `extra.docs_theme.relief` | `silhouette` draws the logo as one flat shape, for logos whose grey tones vanish in the default relief |
| Hero | `extra.docs_theme.hero` | `claim`, `accent` (the half of the claim in the accent colour), `text` and `buttons` |

## Using it in an app

1. Copy `docs-theme/` into the repo root.
2. In `mkdocs.yml`:

   ```yaml
   theme:
     name: material
     custom_dir: docs-theme/overrides
     logo: assets/logo.svg
     font:
       text: Noto Sans
       code: Noto Sans Mono
     palette:
       - scheme: slate
         toggle: { icon: material/weather-sunny, name: Switch to light mode }
       - scheme: default
         toggle: { icon: material/weather-night, name: Switch to dark mode }
     features:
       - navigation.sections
       - navigation.top
       - navigation.footer
       - content.code.copy

   extra:
     docs_theme:
       accent: "#FCC419"
       hero:
         claim: What the app does.
         accent: Why it matters.
         text: One or two sentences on what the app is.
         buttons:
           - { text: Get started, link: getting-started/, primary: true }
           - { text: Source code, link: "https://github.com/OWNER/REPO" }
   ```

3. Remove the app's own `extra.css` and sidebar scripts if they only styled Material. `extra_css` still works for real app additions and loads after the theme.

The hero appears on the home page of every language. A translated home page sets the same keys in its front matter, and only the keys it sets replace the English ones:

```yaml
---
hero:
  claim: Was die App tut.
  accent: Warum das zählt.
---
```

Button links stay relative to the page, so `getting-started/` on the German home page opens the German page. The page's own H1 is hidden under the hero, which already shows the name.

## What is in here

- `overrides/main.html` loads Bree Serif and the theme files, sets the accent and adds the hero and the rail controls.
- `overrides/partials/dt-hero.html` is the landing hero.
- `overrides/partials/dt-sidebar-controls.html` puts the language menu and the dark/light switch at the bottom of the rail. The language list comes from mkdocs-static-i18n, so a site with one language only gets the switch.
- `overrides/partials/languages/no.html` maps the `no` locale to Material's Bokmål strings.
- `overrides/assets/docs-theme/theme.css` and `theme.js` are copied into the built site by MkDocs.

Material allows one `custom_dir`, so an app that needs a template of its own adds it inside `docs-theme/overrides/` under a name the theme does not use.
