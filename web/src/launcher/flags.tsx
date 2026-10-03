// The flags of the languages the app speaks, and no others: the whole set
// would add several megabytes to the APK for flags nobody picks.
const URLS = import.meta.glob(
  "../../node_modules/flag-icons/flags/4x3/{bg,cn,cz,de,dk,ee,es,es-ct,es-ga,es-pv,fi,fr,gb,gr,hr,hu,id,il,in,ir,is,it,jp,kr,lt,lv,my,nl,no,pl,pt,ro,rs,ru,sa,se,si,sk,th,tr,ua,vn}.svg",
  { eager: true, query: "?url", import: "default" }
) as Record<string, string>;

export function flagUrl(code: string): string | undefined {
  return URLS[`../../node_modules/flag-icons/flags/4x3/${code}.svg`];
}

export function Flag({ code }: { code: string }) {
  const src = flagUrl(code);
  return src ? <img src={src} alt="" aria-hidden className="h-[1em] w-[1.33em] shrink-0 rounded-[2px] object-cover" /> : null;
}
