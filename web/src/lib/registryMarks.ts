// The registries most private images come from, each with its logo and the
// host BombVault signs in to. A registry without a fixed host is one people run
// themselves or one a cloud provider hands out per account, and takes its
// address from the form.
//
// The logos are from homarr-labs/dashboard-icons (Apache-2.0), unmodified.
// They are drawn as images, so the ids inside one cannot meet another's.
//
// Trademarks: every mark belongs to its owner and is used only to name the
// service it stands for.

export type RegistryGroup = "public" | "selfHosted" | "cloud";

export interface RegistryKind {
  id: string;
  name: string;
  group: RegistryGroup;
  /** The host a public registry is reached at. */
  host?: string;
  /** What a host of this kind looks like, as the field's placeholder. */
  placeholder?: string;
  /** The brand colour its tile lights up in. */
  tile: string;
  /** A logo in dark ink, which a dark ground has to flip. */
  dark?: boolean;
  mark: string;
}

export const REGISTRY_KINDS: RegistryKind[] = [
  {
    id: "dockerhub",
    name: "Docker Hub",
    group: "public",
    host: "docker.io",
    tile: "#1d63ed",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="-0.07 71.3 511.97 369.5"><path d="M501.4 212.3c-11.5-8-38-11-58.6-7-2.4-20-13.5-37.5-32.7-53l-11-8-7.7 11.5c-9.6 15-14.4 36-13 56 .5 7 2.9 19.5 10.1 30.5-6.7 4-20.7 9-38.9 9H2.3l-1 4c-3.4 20-3.4 82.5 36 130.5 29.8 36.5 74 55 132.1 55 125.9 0 219.1-60.5 262.8-170 17.3.5 54.3 0 73-37.5.5-1 1.4-3 4.8-10.5l1.9-4zM280 71.3h-52.8v50H280zm0 60h-52.8v50H280zm-62.5 0h-52.8v50h52.8zm-62.4 0h-52.8v50h52.8zm-62.5 60H39.8v50h52.8zm62.5 0h-52.8v50h52.8zm62.4 0h-52.8v50h52.8zm62.5 0h-52.8v50H280zm62.4 0h-52.8v50h52.8z" style="fill:#2396ed"/></svg>',
  },
  {
    id: "ghcr",
    name: "GitHub",
    group: "public",
    host: "ghcr.io",
    tile: "#24292f",
    dark: true,
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path d="M256 6.3C114.6 6.3 0 120.9 0 262.3c0 113.3 73.3 209 175 242.9 12.8 2.2 17.6-5.4 17.6-12.2 0-6.1-.3-26.2-.3-47.7-64.3 11.8-81-15.7-86.1-30.1-2.9-7.4-15.4-30.1-26.2-36.2-9-4.8-21.8-16.6-.3-17 20.2-.3 34.6 18.6 39.4 26.2 23 38.7 59.8 27.8 74.6 21.1 2.2-16.6 9-27.8 16.3-34.2-57-6.4-116.5-28.5-116.5-126.4 0-27.8 9.9-50.9 26.2-68.8-2.6-6.4-11.5-32.6 2.6-67.8 0 0 21.4-6.7 70.4 26.2 20.5-5.8 42.2-8.6 64-8.6s43.5 2.9 64 8.6c49-33.3 70.4-26.2 70.4-26.2 14.1 35.2 5.1 61.4 2.6 67.8 16.3 17.9 26.2 40.6 26.2 68.8 0 98.2-59.8 120-116.8 126.4 9.3 8 17.3 23.4 17.3 47.4 0 34.2-.3 61.8-.3 70.4 0 6.7 4.8 14.7 17.6 12.2C438.7 471.3 512 375.3 512 262.3c0-141.4-114.6-256-256-256" style="fill-rule:evenodd;clip-rule:evenodd;fill:#1b1f23"/></svg>',
  },
  {
    id: "gitlab",
    name: "GitLab",
    group: "public",
    host: "registry.gitlab.com",
    tile: "#e24329",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="93.97 97.52 192.05 184.99"><defs><style>.cls-2{fill:#fc6d26}</style></defs><g id="LOGO"><path d="m282.83 170.73-.27-.69-26.14-68.22a6.8 6.8 0 0 0-2.69-3.24 7 7 0 0 0-8 .43 7 7 0 0 0-2.32 3.52l-17.65 54h-71.47l-17.65-54a6.86 6.86 0 0 0-2.32-3.53 7 7 0 0 0-8-.43 6.87 6.87 0 0 0-2.69 3.24L97.44 170l-.26.69a48.54 48.54 0 0 0 16.1 56.1l.09.07.24.17 39.82 29.82 19.7 14.91 12 9.06a8.07 8.07 0 0 0 9.76 0l12-9.06 19.7-14.91 40.06-30 .1-.08a48.56 48.56 0 0 0 16.08-56.04" style="fill:#e24329"/><path d="m282.83 170.73-.27-.69a88.3 88.3 0 0 0-35.15 15.8L190 229.25c19.55 14.79 36.57 27.64 36.57 27.64l40.06-30 .1-.08a48.56 48.56 0 0 0 16.1-56.08" class="cls-2"/><path d="m153.43 256.89 19.7 14.91 12 9.06a8.07 8.07 0 0 0 9.76 0l12-9.06 19.7-14.91S209.55 244 190 229.25c-19.55 14.75-36.57 27.64-36.57 27.64" style="fill:#fca326"/><path d="M132.58 185.84A88.2 88.2 0 0 0 97.44 170l-.26.69a48.54 48.54 0 0 0 16.1 56.1l.09.07.24.17 39.82 29.82L190 229.21Z" class="cls-2"/></g></svg>',
  },
  {
    id: "quay",
    name: "Quay.io",
    group: "public",
    host: "quay.io",
    tile: "#2a6db0",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" x="0" y="0" version="1.1" viewBox="3.08 0.11 39.46 39.46"> <g transform="matrix(.39458 0 0 .39458 1.082 .105)"> <circle cx="55.069" cy="50" r="50" fill="#D71E00"/> <path fill="#C21A00" d="M90.429 14.641c19.529 19.529 19.529 51.19 0 70.718-19.529 19.53-51.192 19.53-70.72 0l70.72-70.718z"/> <path fill="#FFF" d="M59.608 49.99L74.668 81.859 61.838 81.859 46.789 49.99 61.838 18.14 74.668 18.14z"/> <path fill="#B7B7B7" d="M74.668 81.859L89.718 49.99 74.668 18.14 68.258 31.7 76.898 49.99 68.258 68.291z"/> <path fill="#FFF" d="M33.239 49.99L48.299 81.859 35.469 81.859 20.419 49.99 35.469 18.14 48.299 18.14z"/> <path fill="#B7B7B7" d="M48.659 46.04L55.069 32.47 48.299 18.14 41.879 31.71z"/> <path fill="#B7B7B7" d="M41.879 68.291L48.299 81.859 55.069 67.529 48.659 53.961z"/> </g> </svg>',
  },
  {
    id: "codeberg",
    name: "Codeberg",
    group: "public",
    host: "codeberg.org",
    tile: "#2185d0",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><linearGradient id="a" x1="42506.715" x2="42624.223" y1="7642.54" y2="7408.031" gradientTransform="matrix(1.4006 0 0 -1.4006 -59281.402 10836.918)" gradientUnits="userSpaceOnUse"><stop offset="0" style="stop-color:#fff;stop-opacity:0"/><stop offset=".495" style="stop-color:#71c2ff"/><stop offset="1" style="stop-color:#39aaff"/></linearGradient><path d="M260.8 129.9c-1.2 0-2.2.8-2.2 1.7v.3l98.6 369.7c47.7-20.5 88-55.1 115.6-99.1l-210-271.8c-.4-.5-1.2-.8-2-.8" style="opacity:.5;fill:url(#a)"/><path d="M255 10.4C114 11 0 125.4 0 266.4c0 48.1 13.5 95.2 39.1 136l213.4-275.9c1.1-1.4 3.5-1.9 5.5-1.1.6.3 1.1.6 1.5 1.1l213.5 276c25.5-40.8 39.1-87.9 39.1-136 0-141.4-114.6-256-256-256-.4-.1-.7-.1-1.1-.1" style="fill:#2185d0"/></svg>',
  },
  {
    id: "gitea",
    name: "Gitea",
    group: "selfHosted",
    placeholder: "git.example.org",
    tile: "#609926",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" id="main_outline" x="0" y="0" version="1.1" viewBox="-0.05 98.32 512.15 315.48"><style>.st1{fill:#609926}</style><path id="teabag" d="m317.8 376-103.4-49.7c-10.2-4.9-14.6-17.3-9.6-27.5l49.7-103.4c4.9-10.2 17.3-14.6 27.5-9.6 14 6.8 22.1 10.6 22.1 10.6l-.1-88.9 13.6-.1.1 95.4s46.8 19.7 67.7 32.7c3 1.9 8.3 5.5 10.5 11.7 1.7 5 1.6 10.7-.8 15.7l-49.7 103.4c-5 10.3-17.4 14.7-27.6 9.7" style="fill:#fff"/><path d="M502.6 103.7c-3.3-3.3-7.8-3.3-7.8-3.3s-95.5 5.4-144.9 6.5c-10.8.2-21.6.5-32.3.6V203c-4.5-2.1-9-4.3-13.5-6.4 0-29.6-.1-88.9-.1-88.9-23.6.3-72.7-1.8-72.7-1.8s-115.2-5.8-127.7-6.9c-8-.5-18.3-1.7-31.8 1.2-7.1 1.5-27.3 6-43.8 21.9C-8.7 154.8.7 206.7 1.9 214.5c1.4 9.5 5.6 36 25.8 59 37.3 45.7 117.6 44.6 117.6 44.6s9.9 23.5 24.9 45.2c20.4 27 41.3 48 61.7 50.5 51.3 0 153.9-.1 153.9-.1s9.8.1 23-8.4c11.4-6.9 21.6-19.1 21.6-19.1s10.5-11.2 25.2-36.9c4.5-7.9 8.2-15.6 11.5-22.8 0 0 45-95.4 45-188.2-1-28-7.9-33-9.5-34.6M97.7 269.9c-21.1-6.9-30.1-15.2-30.1-15.2S52 243.8 44.2 222.3c-13.4-36-1.1-58-1.1-58s6.8-18.3 31.4-24.4c11.2-3 25.2-2.5 25.2-2.5s5.8 48.4 12.8 76.7c5.9 23.8 20.2 63.3 20.2 63.3s-21.3-2.6-35-7.5m244.6 87.6s-5 11.8-16 12.5c-4.7.3-8.4-1-8.4-1s-.2-.1-4.3-1.7l-92-44.8s-8.9-4.6-10.4-12.7c-1.8-6.6 2.2-14.7 2.2-14.7l44.2-91.1s3.9-7.9 9.9-10.6c.5-.2 1.9-.8 3.7-1.2 6.6-1.7 14.7 2.3 14.7 2.3l90.2 43.7s10.3 4.6 12.5 13.2c1.5 6-.4 11.4-1.5 14-5.2 12.6-44.8 92.1-44.8 92.1" class="st1"/><path d="M261.6 291.2c-6.7.1-12.5 4.7-14.1 11.2-1.5 6.5 1.6 13.3 7.4 16.3 6.3 3.3 14.3 1.5 18.5-4.4 4.2-5.8 3.5-13.8-1.5-18.8l19.5-40c1.2.1 3 .2 5-.4 3.3-.7 5.8-2.9 5.8-2.9 3.4 1.5 7 3.1 10.8 5 3.9 2 7.6 4 10.9 5.9.7.4 1.5.9 2.3 1.5 1.3 1.1 2.8 2.5 3.8 4.5 1.5 4.5-1.5 12.1-1.5 12.1-1.9 6.2-15 33.1-15 33.1-6.6-.2-12.5 4.1-14.4 10.2-2.1 6.6.9 14.1 7.2 17.3 6.4 3.3 14.2 1.4 18.3-4.3 4.1-5.5 3.7-13.3-.9-18.4l4.6-9.2c4.1-8.5 11-24.8 11-24.8.7-1.4 4.6-8.4 2.2-17.3-2-9.3-10.3-13.6-10.3-13.6-9.9-6.4-23.8-12.4-23.8-12.4s0-3.3-.9-5.8-2.3-4.2-3.2-5.1c3.8-7.9 7.7-15.7 11.5-23.6-3.3-1.6-6.6-3.3-9.9-5-3.9 8-7.9 16-11.8 24-5.5-.1-10.5 2.9-13.1 7.7-2.8 5.1-2.2 11.5 1.5 16.1-6.6 13.8-13.3 27.5-19.9 41.1" class="st1"/></svg>',
  },
  {
    id: "forgejo",
    name: "Forgejo",
    group: "selfHosted",
    placeholder: "code.example.org",
    tile: "#d14a00",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" id="Layer_1" x="0" y="0" version="1.1" viewBox="38.5 0.5 135 211"><style>.st3{fill:none;stroke:#d40000;stroke-width:15}</style><g transform="translate(6 6)"><path d="M58 168V70c0-27.6 22.4-50 50-50h20" style="fill:none;stroke:#f60;stroke-width:25"/><path d="M58 168v-30c0-27.6 22.4-50 50-50h20" style="fill:none;stroke:#d40000;stroke-width:25"/><circle cx="142" cy="20" r="18" style="fill:none;stroke:#f60;stroke-width:15"/><circle cx="142" cy="88" r="18" class="st3"/><circle cx="58" cy="180" r="18" class="st3"/></g></svg>',
  },
  {
    id: "harbor",
    name: "Harbor",
    group: "selfHosted",
    placeholder: "harbor.example.org",
    tile: "#4a9c2d",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="1.05 -1.95 359.41 361.66"><defs><linearGradient id="linear-gradient" x1="264.79" x2="267.27" y1="952.39" y2="952.39" gradientTransform="matrix(30.43 0 0 -30.43 -7955.22 29285.75)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#60b932"/><stop offset=".28" stop-color="#60b932"/><stop offset="1" stop-color="#367c34"/></linearGradient><linearGradient xlink:href="#linear-gradient" id="linear-gradient-2" x1="263.77" x2="266.26" y1="955.65" y2="955.65" gradientTransform="matrix(27.21 0 0 -27.21 -7073.85 26169.41)"/><linearGradient xlink:href="#linear-gradient" id="linear-gradient-3" x1="263.28" x2="265.76" y1="953.74" y2="953.74" gradientTransform="matrix(25.75 0 0 -25.75 -6671.13 24812.23)"/><linearGradient xlink:href="#linear-gradient" id="linear-gradient-4" x1="263.77" x2="266.25" y1="953.2" y2="953.2" gradientTransform="matrix(27.1 0 0 -27.1 -7040.9 26102.49)"/><linearGradient xlink:href="#linear-gradient" id="linear-gradient-5" x1="262.73" x2="265.21" y1="954.34" y2="954.34" gradientTransform="matrix(24.4 0 0 -24.4 -6301.36 23521.97)"/><linearGradient id="linear-gradient-6" x1="272.14" x2="274.62" y1="955.15" y2="955.15" gradientTransform="scale(66.09 -66.09)rotate(36.52 1588.153 68.148)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#4596d8"/><stop offset=".2" stop-color="#4596d8"/><stop offset="1" stop-color="#fff"/></linearGradient><linearGradient id="linear-gradient-7" x1="270.65" x2="273.13" y1="952.38" y2="952.38" gradientTransform="scale(77.8 -77.8)rotate(-11.54 -4587.209 1803.323)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#4194d7"/><stop offset=".2" stop-color="#4194d7"/><stop offset="1" stop-color="#fff"/></linearGradient><linearGradient id="linear-gradient-8" x1="270.97" x2="273.45" y1="953.75" y2="953.75" gradientTransform="scale(71.35 -71.35)rotate(10.23 5477.37 -1024.602)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#4194d7"/><stop offset=".33" stop-color="#4496d8"/><stop offset="1" stop-color="#fff"/></linearGradient><clipPath id="clip-path"><path d="m174.4 302.52-68.65-30.26-3.83 43.21 75.5 23.98z" class="cls-1"/></clipPath><clipPath id="clip-path-2"><path d="M119.19 135.38h38.55v32.89h-9.15v-14a10.13 10.13 0 1 0-20.26 0v14h-9.15zm-7.7 72 57.2 25.21-3.94-48.24h7.49v-16.08h-5.55v-32.89h5.55v-9.45l-30.86-30.19v-2.12a2.91 2.91 0 0 0-5.82 0v2.12l-30.86 30.19v9.45h5.56v32.89h-5.56v16.12h8.82z" class="cls-1"/></clipPath><clipPath id="clip-path-3"><path d="m108.14 245.28 63.88 28.16-.96-11.73-61.96-27.3z" class="cls-1"/></clipPath><clipPath id="clip-path-4"><path d="m106.46 264.21 67.23 29.63-.96-11.72-65.31-28.78z" class="cls-1"/></clipPath><clipPath id="clip-path-5"><path d="m110.77 215.48-.96 10.87 60.54 26.68-.95-11.72z" class="cls-1"/></clipPath><clipPath id="clip-path-6"><path d="M313.13 67.59a175.3 175.3 0 0 0-29.75-28.13c-1.57-1.17-3.18-2.3-4.79-3.42L256 59.5l-82.34 85.63 113.41-58.68 29.07-15c-1.01-1.31-1.98-2.62-3.01-3.86" class="cls-1"/></clipPath><clipPath id="clip-path-7"><path d="M353.59 177.61c0-2-.14-4-.22-5.93l-32.21-2.31-147.47-10.58L318.36 209l30.41 10.55c.09-.36.19-.71.28-1.08a173.7 173.7 0 0 0 4.57-39.47v-1.37z" class="cls-1"/></clipPath><clipPath id="clip-path-8"><path d="M348.84 138.61a172.6 172.6 0 0 0-13.84-37.7l-30.94 9.94L175.92 152l142-4.83 32.5-1.11c-.5-2.48-.99-4.98-1.58-7.45" class="cls-1"/></clipPath><style>.cls-1{fill:none}.cls-13{fill:#696566}</style></defs><g id="g12"><path id="path14" fill="#fff" d="M30.89 179a148.87 148.87 0 1 1 148.87 148.85A148.87 148.87 0 0 1 30.89 179"/><g id="g30" clip-path="url(#clip-path)"><g id="g32"><path id="path46" fill="url(#linear-gradient)" d="m174.4 302.52-68.65-30.25-3.82 43.2 75.5 24-3-36.93"/></g></g><g id="g48" clip-path="url(#clip-path-2)"><g id="g50"><path id="path64" fill="url(#linear-gradient-2)" d="M119.19 135.38h38.55v32.89h-9.15v-14a10.13 10.13 0 1 0-20.26 0v14h-9.15zm-7.7 72 57.2 25.21-3.94-48.24h7.49v-16.08h-5.55v-32.89h5.55v-9.45l-30.86-30.19v-2.12a2.91 2.91 0 0 0-5.82 0v2.12l-30.86 30.19v9.45h5.56v32.89h-5.56v16.12h8.82l-2 23"/></g></g><g id="g66" clip-path="url(#clip-path-3)"><g id="g68"><path id="path82" fill="url(#linear-gradient-3)" d="M108.13 245.28 172 273.44l-1-11.72-62-27.31-1 10.87"/></g></g><g id="g84" clip-path="url(#clip-path-4)"><g id="g86"><path id="path100" fill="url(#linear-gradient-4)" d="m106.46 264.21 67.23 29.63-1-11.72-65.31-28.78-1 10.87"/></g></g><g id="g102" clip-path="url(#clip-path-5)"><g id="g104"><path id="path118" fill="url(#linear-gradient-5)" d="m110.77 215.48-1 10.88L170.35 253l-1-11.72-58.62-25.83"/></g></g><path id="path120" d="m304.07 110.85 30.93-9.94c-.11-.22-.21-.45-.32-.66a174.4 174.4 0 0 0-18.55-28.83l-29.07 15a142.7 142.7 0 0 1 16.73 23.87c.1.17.18.35.27.53" class="cls-13"/><path id="path122" d="m321.15 169.37 32.21 2.31a173 173 0 0 0-3-25.59l-32.5 1.11a141 141 0 0 1 3.25 22.17" class="cls-13"/><path id="path124" d="M182 320.81c-78.2 0-141.83-63.62-141.83-141.82S103.82 37.16 182 37.16a140.93 140.93 0 0 1 76.3 22.34L280.85 36A172.86 172.86 0 0 0 182 5.11C86.15 5.11 8.15 83.11 8.15 179s78 173.85 173.85 173.85c81.9 0 150.69-57 169-133.32L320.62 209c-13.8 63.84-70.69 111.83-138.6 111.83" class="cls-13"/><g id="g126" clip-path="url(#clip-path-6)"><g id="g128"><path id="path142" fill="url(#linear-gradient-6)" d="M313.13 67.59a175.3 175.3 0 0 0-29.75-28.13c-1.57-1.17-3.18-2.3-4.79-3.42L256 59.5l-82.34 85.63 113.41-58.68 29.07-15c-1-1.27-2-2.58-3-3.82"/></g></g><g id="g144" clip-path="url(#clip-path-7)"><g id="g146"><path id="path160" fill="url(#linear-gradient-7)" d="M353.59 177.61c0-2-.14-4-.22-5.93l-32.21-2.31-147.47-10.58L318.36 209l30.41 10.55c.09-.36.19-.71.28-1.08a173.7 173.7 0 0 0 4.57-39.47v-1.37"/></g></g><g id="g162" clip-path="url(#clip-path-8)"><g id="g164"><path id="path178" fill="url(#linear-gradient-8)" d="M348.84 138.61a172.6 172.6 0 0 0-13.84-37.7l-30.94 9.94L175.92 152l142-4.83 32.5-1.11c-.48-2.51-1-5-1.56-7.48"/></g></g></g></svg>',
  },
  {
    id: "ecr",
    name: "Amazon ECR",
    group: "cloud",
    placeholder: "123456789012.dkr.ecr.eu-central-1.amazonaws.com",
    tile: "#232f3e",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" id="Layer_1" x="0" y="0" version="1.1" viewBox="0.02 102.6 511.9 306.4"><style>.st1{fill-rule:evenodd;clip-rule:evenodd;fill:#f90}</style><path d="M144.3 214.1c0 6.3.7 11.4 1.9 15.2 1.4 3.7 3.1 7.8 5.4 12.3.9 1.4 1.2 2.7 1.2 3.9 0 1.7-1 3.4-3.2 5.1l-10.7 7.2c-1.5 1-3.1 1.5-4.4 1.5-1.7 0-3.4-.9-5.1-2.4-2.4-2.6-4.4-5.3-6.1-8-1.7-2.9-3.4-6.1-5.3-10-13.3 15.7-30 23.5-50.1 23.5-14.3 0-25.7-4.1-34.1-12.3-8.3-8.2-12.6-19.1-12.6-32.7 0-14.5 5.1-26.2 15.5-35.1S60.8 169 78.4 169c5.8 0 11.7.5 18.1 1.4s12.8 2.2 19.6 3.7v-12.4c0-12.9-2.7-22-8-27.2-5.4-5.3-14.6-7.8-27.8-7.8-6 0-12.1.7-18.4 2.2s-12.4 3.4-18.4 5.8c-2.7 1.2-4.8 1.9-6 2.2s-2 .5-2.7.5c-2.4 0-3.6-1.7-3.6-5.3v-8.3c0-2.7.3-4.8 1.2-6s2.4-2.4 4.8-3.6c6-3.1 13.1-5.6 21.5-7.7 8.3-2.2 17.2-3.2 26.6-3.2 20.3 0 35.1 4.6 44.6 13.8 9.4 9.2 14.1 23.2 14.1 41.9v55.2h.3zM75.2 240c5.6 0 11.4-1 17.5-3.1 6.1-2 11.6-5.8 16.2-10.9 2.7-3.2 4.8-6.8 5.8-10.9s1.7-9 1.7-14.8v-7.2c-4.9-1.2-10.2-2.2-15.7-2.9-5.4-.7-10.7-1-16-1-11.4 0-19.8 2.2-25.4 6.8S51 207.1 51 215.6c0 8 2 14 6.3 18.1 4.1 4.2 10 6.3 17.9 6.3m136.7 18.4c-3.1 0-5.1-.5-6.5-1.7-1.4-1-2.6-3.4-3.6-6.6l-40-131.6c-1-3.4-1.5-5.6-1.5-6.8 0-2.7 1.4-4.3 4.1-4.3h16.7c3.2 0 5.4.5 6.6 1.7 1.4 1 2.4 3.4 3.4 6.6l28.6 112.7 26.6-112.7c.9-3.4 1.9-5.6 3.2-6.6 1.4-1 3.7-1.7 6.8-1.7H270c3.2 0 5.4.5 6.8 1.7 1.4 1 2.6 3.4 3.2 6.6l26.9 114.1 29.5-114.1c1-3.4 2.2-5.6 3.4-6.6 1.4-1 3.6-1.7 6.6-1.7h15.8c2.7 0 4.3 1.4 4.3 4.3 0 .9-.2 1.7-.3 2.7-.2 1-.5 2.4-1.2 4.3l-41 131.6q-1.5 5.1-3.6 6.6c-1.4 1-3.6 1.7-6.5 1.7h-14.6c-3.2 0-5.4-.5-6.8-1.7s-2.6-3.4-3.2-6.8l-26.4-109.8L236.7 250c-.9 3.4-1.9 5.6-3.2 6.8-1.4 1.2-3.7 1.7-6.8 1.7h-14.8zm218.8 4.6c-8.9 0-17.7-1-26.2-3.1-8.5-2-15.2-4.3-19.6-6.8-2.7-1.5-4.6-3.2-5.3-4.8s-1-3.2-1-4.8v-8.7c0-3.6 1.4-5.3 3.9-5.3 1 0 2 .2 3.1.5 1 .3 2.6 1 4.3 1.7 5.8 2.6 12.1 4.6 18.7 6 6.8 1.4 13.5 2 20.3 2 10.7 0 19.1-1.9 24.9-5.6s8.9-9.2 8.9-16.2c0-4.8-1.5-8.7-4.6-11.9s-8.9-6.1-17.2-8.9l-24.7-7.7c-12.4-3.9-21.6-9.7-27.2-17.4-5.6-7.5-8.5-15.8-8.5-24.7 0-7.2 1.5-13.5 4.6-18.9s7.2-10.2 12.3-14c5.1-3.9 10.9-6.8 17.7-8.9 6.8-2 14-2.9 21.5-2.9 3.7 0 7.7.2 11.4.7 3.9.5 7.5 1.2 11.1 1.9 3.4.9 6.6 1.7 9.7 2.7s5.4 2 7.2 3.1c2.4 1.4 4.1 2.7 5.1 4.3 1 1.4 1.5 3.2 1.5 5.6v8c0 3.6-1.4 5.4-3.9 5.4-1.4 0-3.6-.7-6.5-2q-14.55-6.6-32.7-6.6c-9.7 0-17.4 1.5-22.6 4.8s-8 8.2-8 15.2c0 4.8 1.7 8.9 5.1 12.1s9.7 6.5 18.7 9.4l24.2 7.7c12.3 3.9 21.1 9.4 26.4 16.3s7.8 15 7.8 23.8c0 7.3-1.5 14-4.4 19.8-3.1 5.8-7.2 10.9-12.4 15-5.3 4.3-11.6 7.3-18.9 9.5-8 2.5-16 3.7-24.7 3.7" style="fill:#000"/><path d="M462.9 345.7c-56 41.4-137.4 63.3-207.4 63.3-98.1 0-186.5-36.3-253.2-96.6-5.3-4.8-.5-11.2 5.8-7.5 72.2 41.9 161.3 67.3 253.4 67.3 62.2 0 130.4-12.9 193.3-39.5 9.3-4.2 17.3 6.2 8.1 13" class="st1"/><path d="M486.2 319.2c-7.2-9.2-47.3-4.4-65.6-2.2-5.4.7-6.3-4.1-1.4-7.7 32-22.5 84.6-16 90.8-8.5 6.1 7.7-1.7 60.3-31.7 85.5-4.6 3.9-9 1.9-7-3.2 6.9-16.9 22.1-54.9 14.9-63.9" class="st1"/></svg>',
  },
  {
    id: "gar",
    name: "Google Artifact Registry",
    group: "cloud",
    placeholder: "europe-west3-docker.pkg.dev",
    tile: "#4285f4",
    mark:
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="40.28 64.1 431.68 383.8"><path d="m322.4 140.9-44.1-76.8H162.6c-10.3 0-19.9 5.5-24.9 14.5L44.1 241.7c-5.1 8.9-5.1 19.7 0 28.7l35.6 62.1 44-76.4 66.2-115.2z" style="fill:#f8bb16"/><path d="M468.2 241.7 374.6 78.6c-5.2-8.9-14.7-14.5-24.9-14.5h-71.4l44.1 76.8 66.2 115.2-66.2 115.2h88l57.8-100.9c5-8.9 5-19.8 0-28.7" style="fill:#ea4334"/><path d="M410.3 371.2H189.7L123.5 256l-44 76.4 58 101c5.2 8.9 14.7 14.5 24.9 14.5h187.1c10.3 0 19.9-5.5 24.9-14.5z" style="fill:#0074bc"/><path d="M322.2 371.2H189.6L123.5 256l66.2-115.2h132.5L388.6 256zM256 198.5c-31.9 0-57.6 25.7-57.6 57.6s25.7 57.6 57.6 57.6 57.6-25.7 57.6-57.6-25.8-57.6-57.6-57.6" style="fill:#e2e3e4"/></svg>',
  },
  {
    id: "acr",
    name: "Azure Container Registry",
    group: "cloud",
    placeholder: "myname.azurecr.io",
    tile: "#0078d4",
    mark:
      '<svg id="e9f7ba73-56b8-4864-96ff-dce80bef729a" xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 18 18"><defs><linearGradient id="bdf27dc6-025c-4805-8bfd-3a05defd3fbd" x1="8.637" y1="-1.991" x2="8.637" y2="16.739" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#5ea0ef" /><stop offset="1" stop-color="#0078d4" /></linearGradient><linearGradient id="fcd6c6c7-e2eb-4a32-b914-3a8485585a1b" x1="12.96" y1="8.561" x2="12.96" y2="6.141" gradientTransform="matrix(1, 0, 0, -1, 0, 20)" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#333132" /><stop offset="1" stop-color="#5b5a5c" /></linearGradient></defs><path d="M7.43,8.178l2.589-1.573,7.255,2.731A3.664,3.664,0,0,0,16.23,7.49l-.01-.05A4.194,4.194,0,0,0,14,6.32a4.91,4.91,0,0,0-5.1-4.7A5.071,5.071,0,0,0,4.06,4.91,4.621,4.621,0,0,0,0,9.39a4.73,4.73,0,0,0,4.89,4.54H7.43Z" fill="url(#bdf27dc6-025c-4805-8bfd-3a05defd3fbd)" /><polygon points="10.07 7.159 10.08 11.439 17.99 13.119 17.99 10.139 10.07 7.159" fill="#767676" /><polyline points="10.07 7.159 7.93 8.459 7.93 12.439 10.08 11.439" fill="#999" /><polygon points="13.68 11.499 14.4 11.699 14.4 9.369 13.68 9.119 13.68 11.499" fill="#a3a3a3" /><polygon points="12.96 8.889 12.24 8.629 12.24 11.119 12.96 11.319 12.96 8.889" fill="#a3a3a3" /><polygon points="15.12 11.889 15.82 12.079 15.84 9.859 15.12 9.619 15.12 11.889" fill="#a3a3a3" /><polygon points="10.81 10.749 11.53 10.929 11.53 8.399 10.81 8.159 10.81 10.749" fill="#a3a3a3" /><polygon points="17.27 10.349 16.55 10.099 16.55 12.269 17.27 12.469 17.27 10.349" fill="#a3a3a3" /><path id="ef0d1b54-a1e7-4cb9-a4e5-8a8518e7c127" d="M8.66,11.369l-.36.21V8.749l.36-.19Zm.71-3.22L9,8.389v2.75l.37-.2Z" fill="#b3b3b3" /><polygon points="17.99 13.119 15.83 13.859 7.93 12.439 10.08 11.439 17.99 13.119" fill="url(#fcd6c6c7-e2eb-4a32-b914-3a8485585a1b)" /><polygon points="17.99 16.169 10.04 17.679 10.08 12.089 17.99 13.559 17.99 16.169" fill="#767676" /><polygon points="10.81 16.759 10.81 13.209 11.53 13.299 11.53 16.639 10.81 16.759" fill="#a3a3a3" /><polygon points="12.96 16.399 12.24 16.529 12.24 13.389 12.96 13.499 12.96 16.399 12.96 16.399" fill="#a3a3a3" /><polygon points="13.68 16.289 13.68 13.569 14.4 13.659 14.4 16.159 13.68 16.289" fill="#a3a3a3" /><polygon points="15.83 15.909 15.12 16.039 15.12 13.749 15.83 13.859 15.83 15.909" fill="#a3a3a3" /><polygon points="17.29 15.679 16.55 15.809 16.55 13.929 17.24 14.019 17.29 15.679" fill="#a3a3a3" /><path id="b9f25eb4-4c88-45c2-bc4d-f992757d7e0e" d="M7.93,16.4v-3.26l2.16-1v5.6Z" fill="#999" /><path id="eb9200a7-4693-4427-bdae-b33ce90ff7f2" d="M8.61,16.389l-.32-.16v-2.76l.32-.15Zm.77-3.45-.38.19v3.48l.37.19v-3.86Z" fill="#b3b3b3" /></svg>',
  },
];

/**
 * registryKind finds the kind an entry was saved with, and for an entry that
 * carries none the kind its host belongs to.
 */
export function registryKind(host: string, kind?: string): RegistryKind | undefined {
  const byId = REGISTRY_KINDS.find((k) => k.id === kind);
  if (byId) return byId;
  const h = host.trim().toLowerCase();
  if (!h) return undefined;
  return (
    REGISTRY_KINDS.find((k) => k.host === h) ??
    REGISTRY_KINDS.find((k) => k.group === "cloud" && k.placeholder && h.endsWith(k.placeholder.replace(/^[^.]*/, "")))
  );
}
