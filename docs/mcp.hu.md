# MCP-kiszolgáló

A BombVault beépített kiszolgálót kínál a Model Context Protocolhoz (MCP), ahhoz a protokollhoz, amelyen keresztül az olyan MI-asszisztensek, mint a Claude Code és a Claude Desktop, külső eszközöket érnek el. Ezen át egy asszisztens elolvashatja, hogyan állnak a mentéseid, és ha engeded, elindíthat egy mentést, vagy megszakíthat egy olyat, amelyet ő maga indított. A kiszolgáló ki van kapcsolva, amíg nem hozol létre kulcsot: aktív kulcs nélkül a `/mcp` végpont mindenre `404`-gyel válaszol.

## Mit tehet és mit nem egy asszisztens {#tools}

| Eszköz | Mit csinál | Fajta |
|---|---|---|
| `get_health` | Verzió, a példány neve, fut-e mentés, és mit tehet ez a kulcs | olvasás |
| `get_status` | Védettségi állapot tartományonként: utolsó sikeres mentés, várt időköz, ellenőrzések és off-site vizsgálatok, következő ütemezett futások | olvasás |
| `get_coverage` | Mit véd a BombVault és mit nem, mindegyiknél az okkal | olvasás |
| `list_items` | Minden védett konténer, VM és mappakészlet, a flash meghajtó és az alkalmazás beállításai, ütemezéssel, azzal, hogy egy mentés mit állít le, az utolsó mentéssel és annak idejével; az adatbázis-konténerek az utolsó dumpot is megadják; a ZFS-adatkészletek is szerepelnek, a legutóbbi ellenőrzésük eredményével | olvasás |
| `list_runs` | Futási előzmények, a legújabbak elöl, tartomány, elem, állapot, fajta és idő szerint szűrhetően | olvasás |
| `list_restore_points` | Egy elem visszaállítási pontjai az elsődleges tárolójából, konténernél az adatbázis-dumpjai is; egy ZFS-adatkészletnek mentésenként egy visszaállítási pontja van, az alatta lévő minden adatkészlet pillanatképével | olvasás |
| `get_activity` | Mi fut éppen, fázissal és százalékkal | olvasás |
| `get_storage_stats` | Egy tartomány elsődleges tárolójának méretelőzménye és heti növekedése | olvasás |
| `list_anomalies` | Szokatlan mentések, amelyeket a BombVault észrevett, állapot, súlyosság és tartomány szerint szűrhetők, a nyitott tételek összesítésével | olvasás |
| `get_anomaly` | Egy ilyen észlelés, a nyugtázáskor hagyott megjegyzéssel | olvasás |
| `start_backup` | Azonnal elmenti egy elem adatait | indítás |
| `start_domain_backup` | Egy tartomány minden védett elemét menti | indítás |
| `start_backup_everything` | Lefuttatja a Backup Everything menetet | indítás |
| `cancel_backup` | Megszakít egy futó mentést, amelyet ez a kulcs indított | megszakítás |

Ezek a webes felületen maradnak: bármilyen visszaállítás (beleértve egy adatbázis-dump letöltését, mentését vagy importálását is), mentések törlése, prune, unlock, ellenőrzések és gyakorlatok, off-site replikáció, beállítások, hitelesítő adatok és MCP-kulcsok, valamint olyan mentés megszakítása, amelyet az ütemezés, a webes felület vagy egy másik kulcs indított. Ugyanez vonatkozik egy anomália nyugtázására vagy vártként megjelölésére, ami az **Anomáliák** oldalon történik. Az ok: az eszközök válaszai a kiszolgálódról származó neveket és hibaüzeneteket tartalmaznak, és bármelyikükben lehet olyan szöveg, amelyet az asszisztens irányítására írtak. Egy asszisztens, amely bedől ennek, legrosszabb esetben elindít egy mentést az alábbi korlátokon belül, vagy megszakít egyet, amelyet maga indított.

Ha egy elem elsődleges tárolója máshol van (S3, REST, SFTP, rclone), a `list_restore_points` kapcsolódik hozzá, és a hívás eltarthat egy ideig. Az off-site másolatok MCP-n keresztül nem listázhatók.

## Mit csinál egy elindított mentés {#starting-backups}

Az asszisztens mentése ugyanaz, mint amit a webes felület indít. Egy futó konténer leáll, amíg a mentése be nem fejeződik, azokkal a konténerekkel együtt, amelyek beállítás szerint vele együtt állnak le. Egy "graceful" módszerű VM leáll, majd újraindul. Egy ZFS-adatkészlet a pillanatképe készítésének idejére leállítja a hozzá beállított konténereket. A mappakészletek, a flash meghajtó és a beállítások tovább futnak. Utána a BombVault alkalmazza a megőrzési szabályt, és esetleg átmásol az off-site tárolóba. A `list_items` megmondja az asszisztensnek, mit állít le egy elem és mennyi ideig tartott az utolsó mentése, az eszközök leírása pedig arra kéri, hogy ezt mondja el neked, mielőtt bármit elindít.

Mivel egy mentés leállít dolgokat és kiszorítja a régi visszaállítási pontokat, az MCP-n keresztüli indítások korlátozottak:

- Óránként és kulcsonként 12 elindított mentés.
- 15 perc ugyanannak az elemnek, ugyanannak a tartománynak vagy a Backup Everythingnek két MCP-indítása között.
- Legfeljebb 4 MCP-indítás ugyanarra az elemre 24 óra alatt.
- **Megőrzésvédelem.** Ha egy tartomány rögzített számú visszaállítási pontot tart meg (csak "az utolsó N megtartása", napi, heti vagy havi szabály nélkül, helyben vagy egy off-site célon), minden új mentés kiszorítja a legrégebbit. A BombVault ilyenkor elutasítja egy elem MCP-indítását, ha a legutóbbi N-1 sikeres mentését mind MCP-n keresztül indították. Így a megtartott halmazban mindig marad legalább egy visszaállítási pont, amelyet az ütemezés vagy te hoztál létre. "Az utolsó 1 megtartása" beállításnál egy asszisztens egyáltalán nem mentheti azt az elemet. A következő ütemezett mentés újra helyet csinál.

Egy tartomány vagy a Backup Everything indítása kihagyja azokat az elemeket, amelyeket valamelyik korlát visszatart, és megnevezi őket a válaszában. Egyik korlát sem vonatkozik a webes felületre és az ütemezésre. Az óránkénti keret a memóriában van, így a BombVault újraindítása lenullázza.

## Bekapcsolás {#switch-on}

1. Nyisd meg a **Beállítások, Rendszer, MCP-kiszolgáló** részt, és kattints az **Új kulcs** gombra.
2. Adj a kulcsnak olyan nevet, amely megmondja, hol használod, például "Claude Code a laptopon". Kliensenként egy kulccsal visszavonhatsz egyet anélkül, hogy a többihez hozzányúlnál.
3. Hagyd bekapcsolva a **Mentések indításának engedélyezése** kapcsolót, vagy kapcsold ki egy olyan kulcsnál, amelynek csak olvasnia kell. Később a kulcs sorában módosíthatod, és a változás az asszisztens következő kérésétől él, újracsatlakozás nélkül.
4. Kattints a **Kulcs létrehozása** gombra. A kulcs egyszer jelenik meg. A BombVault csak egy ujjlenyomatot őriz meg belőle, és nem tudja újra megmutatni, ezért másold ki most, vagy vedd az alatta lévő részletek egyikét, amely ekkor a valódi kulcsot tartalmazza.

Bejelentkezési jelszó nélkül maga a webes felület is nyitva áll mindenki előtt a hálózatodon, és aki meg tudja nyitni, kulcsot is létrehozhat. A kártya ezt jelzi. Ha a BombVaultot nyilvánosnak tűnő néven nyitod meg (például `bombvault.example.com` egy fordított proxy mögött), és nincs bejelentkezési jelszó, arról a címről nem lehet kulcsot létrehozni vagy cserélni, így egyetlen internetes weboldal sem veheti rá a böngésződet, hogy létrehozzon egyet. Állíts be bejelentkezési jelszót, vagy nyisd meg a BombVaultot az IP-címén vagy egy helyi néven, például `tower` vagy `tower.local`.

## Kliens csatlakoztatása {#clients}

A kártya kész részleteket mutat ahhoz a címhez, amelyen megnyitottad: válaszd ki a klienst, és másold ki a részletet. A szakasz többi része elmagyarázza, mit csinálnak a részletek, és megadja azokat a formákat, amelyeket a kártya nem mutat.

### Claude Code {#claude-code}

Futtasd le egyszer a kártya parancsát egy terminálban. Olyan tanúsítvánnyal, amelyben a számítógéped megbízik, így néz ki:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

A kapcsolatot a Claude Code-ban a `/mcp` paranccsal ellenőrizheted. A `--scope user` a kulcsot a felhasználói beállításaidba teszi, nem egy projektfájlba.

A parancs tartalmazza a kulcsot, és a shelled megőrizheti az előzményei között. Ezt elkerülheted egy `.mcp.json` fájllal a projekt mappájában és a kulccsal egy környezeti változóban. A Claude Code a fájl olvasásakor behelyettesíti a `${BOMBVAULT_MCP_KEY}` értékét:

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

Állítsd be a `BOMBVAULT_MCP_KEY` változót ott, ahol a Claude Code indul, például a shell profiljában, szövegszerkesztővel és nem a parancssorba gépelve. Soha ne commitolj olyan `.mcp.json` fájlt, amelybe a kulcs bele van írva.

A BombVault saját tanúsítványával (lásd [TLS és tanúsítványok](#tls)) a kártya parancsa ehelyett az `mcp-remote` programot indítja, és a Node.js-t a letöltött tanúsítványra irányítja:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Az egyszeres idézőjelek megakadályozzák, hogy a shell kifejtse a változót; ezt az `mcp-remote` maga végzi el. Ugyanez a forma működik a `.mcp.json` fájlban is: használd az alábbi Claude Desktop bejegyzést, és hagyd ki a `BOMBVAULT_MCP_KEY` változót az `env` részéből, így a kulcs a környezetedből jön.

### Claude Desktop {#claude-desktop}

A Claude Desktop az `mcp-remote` programon keresztül éri el a BombVaultot, amelyhez Node.js kell azon a számítógépen. Nyisd meg a konfigurációs fájlt a Claude Desktopban a **Settings, Developer, Edit Config** útvonalon. Windowson a `%APPDATA%\Claude\claude_desktop_config.json`, macOS-en a `~/Library/Application Support/Claude/claude_desktop_config.json` helyen van. Add hozzá a kártya bejegyzését a `"mcpServers"` részen belül, a már ott lévő kiszolgálók mellé, és indítsd újra a Claude Desktopot:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- A `NODE_EXTRA_CA_CERTS` csak a BombVault saját tanúsítványa miatt van ott. Olyan tanúsítvány mögött, amelyben a számítógéped már megbízik, hagyd el.
- A `--allow-http` csak egyszerű `http://` címnél kerül bele.
- A fejléc `X-API-Key:${BOMBVAULT_MCP_KEY}` alakban áll, szóköz nélkül a kettőspont után, a kulccsal az `env` részben. Egyes rendszereken az `mcp-remote` az első szóköznél kettévágja a `--header` értékét, és egy szóköz után írt kulcs elveszne.

### Saját csatlakozók a Claude beállításaiban {#custom-connectors}

Azok a csatlakozók (connectors), amelyeket magának a Claude-nak a beállításaiban adsz hozzá (a claude.ai oldalon és a Claude Desktop csatlakozólistájában), még nem támogatottak. Ezeket az Anthropic felhőjéből érik el, ezért nyilvános HTTPS-címre van szükségük, és OAuth-on keresztül jelentkeznek be. Rögzített kulcsot nem tudnak küldeni, a BombVault pedig csak rögzített kulcsokat kínál, OAuth-bejelentkezést nem. Ha miattuk kitennéd a BombVaultot az internetre, az sem segítene. Használd a Claude Code-ot, vagy a Claude Desktopot az `mcp-remote` programon keresztül, ahogy fent.

### Más kliensek {#other-clients}

Bármelyik kliens megfelel, amely Streamable HTTP-t beszél:

- URL: a webes felület címe és utána `/mcp`, például `https://192.168.1.10:3443/mcp`.
- A kulcs az `Authorization: Bearer <key>` vagy az `X-API-Key: <key>` fejlécben. Ha mindkettő jön, ugyanazt a kulcsot kell tartalmazniuk.
- `POST` kérés `Content-Type: application/json` és `Accept: application/json, text/event-stream` fejlécekkel.
- Kérésenként egy JSON-RPC üzenet; a kötegeket (batch) elutasítja.
- Protokollverziók: 2026-07-28, 2025-11-25, 2025-06-18 és 2025-03-26.

## TLS és tanúsítványok {#tls}

A BombVault saját maga által kiállított tanúsítvánnyal szolgálja ki a HTTPS-t, és ez a tanúsítvány kezdetben csak a `localhost`, `127.0.0.1` és `::1` neveket tartalmazza. A Claude Code és az `mcp-remote` helyi hálózati címen elutasítja. A megoldások abban a sorrendben, amely a legtöbb Unraid-telepítéshez illik:

1. **Add hozzá a címet az MCP-kártyán.** Ha a kártyát HTTPS-en olyan címen nyitod meg, amelyet a tanúsítvány nem tartalmaz, a kártya jelzi ezt, és felkínálja a **Cím felvétele a tanúsítványba** gombot. A BombVault ekkor újra kiállítja a tanúsítványát ezzel a címmel (a böngésződ még egyszer figyelmeztet, ahogy először). Ezután kattints a **Tanúsítvány letöltése** gombra; a részletek a `NODE_EXTRA_CA_CERTS` értékét a letöltött fájlra állítják, így a kliens pontosan ebben a tanúsítványban bízik meg.
2. **Fordított proxy megbízható tanúsítvánnyal** (Nginx Proxy Manager, SWAG, Caddy, Traefik). A kliens ekkor a proxy tanúsítványát látja, és nincs szüksége semmi másra, a kártya pedig nem figyelmeztet a BombVault sajátjára.
3. **Tailscale.** A konténer elé tett `tailscale serve` vagy az Unraid Tailscale-integrációja egy `ts.net` nevet ad megbízható tanúsítvánnyal.
4. **`HTTP_ONLY=true`**, csak TLS-t lezáró proxy mögött vagy olyan hálózaton, amelyben teljesen megbízol. Az egész webes felületet egyszerű HTTP-re kapcsolja, a konténer beállításainak módosítását igényli, és titkosítatlanul küldi a kulcsot.

Soha ne állítsd be a `NODE_TLS_REJECT_UNAUTHORIZED=0` értéket. Ez kikapcsolja a tanúsítványellenőrzést mindenre, amivel az adott Node.js-folyamat kommunikál.

A fordított proxynak tovább kell adnia az `Authorization` (vagy `X-API-Key`) fejlécet, amit a proxyk meg is tesznek, hacsak másképp nem utasítják őket, és nem pufferelheti vagy írhatja át a `/mcp` útvonalat. Egy location blokk Nginxhez vagy Nginx Proxy Managerhez, amely a BombVault tanúsítványát is ellenőrzi:

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

Proxy mögött minden kérés a proxy címét viseli. Egyetlen rosszul beállított kliens öt hibás kulcsa ilyenkor egy percre kizárja az összes MCP-klienst a proxy mögött. Add meg a proxyt a `TRUSTED_PROXY` változóban (lásd [Beállítások](configuration.md)), hogy kliensenként számoljon.

## Biztonsági modell {#security}

- Aktív kulcs nélkül a `/mcp` `404`-gyel válaszol.
- Nincs kivételezett cím. A `localhost`, az Unraid-gazdagép, egy fordított proxy vagy a `tailscale serve` felől érkező kérésekhez ugyanúgy kulcs kell, mint bármely máshoz, akkor is, ha a webes felületnek nincs bejelentkezési jelszava.
- A kulcsokat csak ujjlenyomatként tárolja, egyszer jeleníti meg, és átnevezhetők, cserélhetők, visszavonhatók. Legfeljebb 10 aktív kulcs, mindegyik saját **Indíthat mentéseket** kapcsolóval.
- Minden létrehozás, csere, jogosultság-módosítás és visszavonás értesítést küld az értesítési csatornáidon a címmel együtt, ahonnan érkezett, hacsak nincsenek kikapcsolva az értesítések.
- Címenként percenként 5 hibás kulcs, utána `429`. Kulcsonként percenként 120 kérés és óránként 12 elindított mentés, ehhez jön a fent leírt várakozási idő és a megőrzésvédelem.
- Egy másik originről érkező böngészőoldal kéréseit elutasítja.
- Amíg nincs bejelentkezési jelszó, nyilvánosnak tűnő gazdagépnévről nem lehet kulcsot létrehozni.
- Minden mentés, amelyet egy asszisztens indít, és az ebből következő prune és off-site futások "MCP-n keresztül" jelölést kapnak a kulcs nevével a tevékenységnaplóban, a hibapanelen és a mentés értesítésében.
- Minden eszközhívás bekerül a konténer naplójába a kulcs azonosítójával és utolsó négy karakterével (a nevével soha), és a `/metrics` számolja (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Egy konfigurációs mentés visszaállítása minden kulcsot visszavon, mert a visszaállított adatbázis olyan kulcsokat is tartalmazhat, amelyeket a mentése után vontál vissza. Utána hozz létre új kulcsokat.
- Egy kulcs megszűnik működni, ha az `APP_KEY` megváltozik (újratelepítés vagy visszaállítás egy másik konténerbe). A kártya ezt észleli és megjelöli a kulcsot, a **Kulcs cseréje** pedig újra érvényes titkot ad neki.
- Úgy bánj a kulccsal, mint egy jelszóval. A Claude Code és a Claude Desktop nyílt szövegként tárolja a beállításaiban. Olyan számítógépen, amelyben kevésbé bízol, inkább csak olvasásra jogosult kulcsot használj.

## Mi hagyja el a gépet {#privacy}

Mindaz, amit egy asszisztens elolvas, a mögötte álló MI-szolgáltatóhoz kerül: az elemek nevei, az ütemezések, a futási előzmények hibaüzenetekkel, a visszaállítási pontok azonosítói és időpontjai, az adatbázismotorok nevei és a dumpok mérete, a folyamatban lévő tevékenység, a tárolási adatok, a lefedettség és az állapot. A BombVault eltávolítja a gazdagép útvonalait, a tárolók helyét, a gazdagépneveket, a hitelesítő adatokat, a hook-parancsokat és a kulcsokat, mielőtt bármi kimenne.

## Hibaelhárítás {#troubleshooting}

| Amit látsz | Amit jelent |
|---|---|
| `404` | Nincs aktív kulcs, vagy rossz az útvonal, például `/api/mcp`. A végpont a `/mcp`. |
| `401` | A kulcs hiányzik, elgépelték, visszavonták vagy lecserélték. Lehet, hogy egy proxy eldobja az `Authorization` fejlécet (próbáld az `X-API-Key` fejlécet). Ha a kártya a kulcsot már nem érvényesnek jelöli, megváltozott az `APP_KEY`: cseréld le a kulcsot. |
| `403` | A kérés egy másik originről érkező böngészőoldalról jött. Használj asztali vagy parancssori klienst. |
| `405` GET esetén | Normális. A végpont csak `POST` kérést fogad. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | A kliens túl régi a Streamable HTTP-hez. Frissítsd. |
| `400` "batch requests are not accepted" | A kliens JSON-RPC kötegeket küld. Kérésenként egy üzenetet küldj. |
| `429` | Túl sok hibás kulcs erről a címről, vagy egy kulccsal több mint 120 kérés percenként. Várj egy percet, és nézd meg, nem ragadt-e az asszisztens egy ciklusba. |
| "certificate", "self-signed" vagy "unable to verify" szövegű hibák | A kliens nem bízik a BombVault tanúsítványában. Lásd [TLS és tanúsítványok](#tls). |
| `busy` | Egy másik mentés vagy karbantartási feladat foglalja a tartományt. Próbáld újra, ha végzett. |
| `cooldown` | Ezt az elemet, ezt a tartományt vagy a Backup Everythinget kevesebb mint 15 perce indították MCP-n keresztül. |
| `retention_guard` | Még egy MCP-mentés után "az utolsó N megtartása" ablakban csak MCP-ből származó visszaállítási pontok maradnának. A következő ütemezett mentés helyet csinál, vagy indítsd a webes felületen. |
| `rate_limited` | A kulcs elhasználta az erre az órára jutó 12 indítását. |
| `not_permitted` indításnál | A kulcs csak olvashat. Kapcsold be az **Indíthat mentéseket** kapcsolót a kártyán; újracsatlakozás nem kell. Megszakításnál azt jelenti, hogy a futást nem ez a kulcs indította. |
| `domain_off` | Ez a mentésfajta ki van kapcsolva a beállításokban. |
| `not_found` | A BombVault nem védi ezt az elemet. Előbb vedd fel a webes felületen; az MCP soha nem hoz létre beállítást. |

Ne állítsd be a konténeren az `MCPGODEBUG` környezeti változót. Megváltoztatja az MCP-könyvtár működését, és egy hibás érték indításkor leállítja a BombVaultot, mielőtt egyetlen naplósort is írna.
