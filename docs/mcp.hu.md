# MCP-kiszolgáló

A BombVault beépített kiszolgálót tartalmaz a Model Context Protocolhoz (MCP), amelyen keresztül az olyan MI-asszisztensek, mint a Claude Code és a Claude Desktop, külső eszközöket érnek el. Ezen át egy asszisztens elolvashatja, hogy állnak a mentéseid, és ha engeded, elindíthat egy mentést, vagy megszakíthat egy általa indítottat. Ki van kapcsolva, amíg nem hozol létre kulcsot vagy nem kapcsolod be az [OAuth-bejelentkezést](#oauth): addig a `/mcp` végpont mindenre `404`-et válaszol.

## Mit tehet és mit nem egy asszisztens {#tools}

| Eszköz | Mit csinál | Fajta |
|---|---|---|
| `get_health` | Verzió, a példány neve, fut-e mentés, és mit tehet ez a kulcs | olvasás |
| `get_status` | Védettségi állapot tartományonként: utolsó sikeres mentés, várt időköz, ellenőrzések és off-site vizsgálatok, következő ütemezett futások | olvasás |
| `get_coverage` | Mit véd a BombVault és mit nem, mindegyiknél az okkal | olvasás |
| `list_items` | Minden védett konténer, VM és mappakészlet, a flash meghajtó és az alkalmazás beállításai, ütemezéssel, azzal, hogy egy mentés mit állít le, az utolsó mentéssel és annak idejével; az adatbázis-konténerek az utolsó dumpot is megadják; a ZFS-adatkészletek is szerepelnek, a legutóbbi ellenőrzésük eredményével; a legutóbbi mentés óta más beállításokkal újra létrehozott konténer felsorolja a változásokat | olvasás |
| `list_runs` | Futási előzmények, a legújabbak elöl, tartomány, elem, állapot, fajta és idő szerint szűrhetően; a lassú mentés, amelyet egyetlen dolog fékezett, megnevezi azt | olvasás |
| `list_restore_points` | Egy elem visszaállítási pontjai az elsődleges tárolójából, konténernél az adatbázis-dumpjai is; egy ZFS-adatkészletnek mentésenként egy visszaállítási pontja van, az alatta lévő minden adatkészlet pillanatképével | olvasás |
| `get_activity` | Mi fut éppen, fázissal és százalékkal | olvasás |
| `get_storage_stats` | Egy tartomány elsődleges tárolójának méretelőzménye és heti növekedése, valamint a foglalt, szabad és teljes hely minden tárolójának lemezén vagy távoli helyén | olvasás |
| `get_size_breakdown` | Mely mappák és fájlok foglalnak helyet egy konténer, VM vagy mappakészlet legújabb mentésében, és ebből mennyit adott hozzá a legutóbbi mentés | olvasás |
| `list_anomalies` | Anomáliák, amelyeket a BombVault a mentésekben észrevett, állapot, súlyosság és tartomány szerint szűrhetők, a nyitott tételek összesítésével | olvasás |
| `get_anomaly` | Egy ilyen észlelés, a nyugtázáskor hagyott megjegyzéssel | olvasás |
| `start_backup` | Azonnal elmenti egy elem adatait | indítás |
| `start_domain_backup` | Egy tartomány minden védett elemét menti | indítás |
| `start_backup_everything` | Lefuttatja a Backup Everything menetet | indítás |
| `cancel_backup` | Megszakít egy futó mentést, amelyet ez a kulcs indított | megszakítás |

Ezek a webes felületen maradnak: bármilyen visszaállítás (beleértve egy adatbázis-dump letöltését, mentését vagy importálását is), mentések törlése, prune, unlock, ellenőrzések és gyakorlatok, off-site replikáció, beállítások, hitelesítő adatok és MCP-kulcsok, valamint olyan mentés megszakítása, amelyet az ütemezés, a webes felület vagy egy másik kulcs indított. Ugyanez vonatkozik egy anomália nyugtázására vagy vártként megjelölésére, ami az **Anomáliák** oldalon történik. Az ok: az eszközök válaszai a kiszolgálódról származó neveket és hibaüzeneteket tartalmaznak, és bármelyikükben lehet olyan szöveg, amelyet az asszisztens irányítására írtak. Egy asszisztens, amely bedől ennek, legrosszabb esetben elindít egy mentést az alábbi korlátokon belül, vagy megszakít egyet, amelyet maga indított.

Ha egy elem elsődleges tárolója máshol van (S3, REST, SFTP, rclone), a `list_restore_points` kapcsolódik hozzá, és a hívás eltarthat egy ideig. Az off-site másolatok MCP-n keresztül nem listázhatók. Hogy az anomáliaellenőrzések mit néznek, azt a [Funkciók](features.md) oldal írja le, hogy egy ZFS-elem adatkészletenként egy pillanatképet tart, azt pedig a [ZFS-adatkészletek](zfs-datasets.md#contents) oldal.

## Mit csinál egy elindított mentés {#starting-backups}

Az asszisztens mentése ugyanaz, mint amit a webes felület indít. Egy futó konténer leáll, amíg a mentése be nem fejeződik, azokkal a konténerekkel együtt, amelyek beállítás szerint vele együtt állnak le. Egy "graceful" módszerű VM leáll, majd újraindul. Egy ZFS-adatkészlet a pillanatképe készítésének idejére leállítja a hozzá beállított konténereket. A mappakészletek, a flash meghajtó és a beállítások tovább futnak. Utána a BombVault alkalmazza a megőrzési szabályt, és esetleg átmásol az off-site tárolóba. A `list_items` megmondja az asszisztensnek, mit állít le egy elem és mennyi ideig tartott az utolsó mentése, az eszközök leírása pedig arra kéri, hogy ezt mondja el neked, mielőtt bármit elindít.

Mivel egy mentés leállít dolgokat és kiszorítja a régi visszaállítási pontokat, az MCP-n keresztüli indítások korlátozottak:

- Óránként és kulcsonként 12 elindított mentés.
- 15 perc ugyanannak az elemnek, ugyanannak a tartománynak vagy a Backup Everythingnek két MCP-indítása között.
- Legfeljebb 4 MCP-indítás ugyanarra az elemre 24 óra alatt.
- **Megőrzésvédelem.** Ha egy tartomány rögzített számú visszaállítási pontot tart meg (csak "az utolsó N megtartása", napi, heti vagy havi szabály nélkül, helyben vagy egy off-site célon), minden új mentés kiszorítja a legrégebbit. A BombVault ilyenkor elutasítja egy elem MCP-indítását, ha a legutóbbi N-1 sikeres mentését mind MCP-n keresztül indították. Így a megtartott halmazban mindig marad legalább egy visszaállítási pont, amelyet az ütemezés vagy te hoztál létre. "Az utolsó 1 megtartása" beállításnál egy asszisztens egyáltalán nem mentheti azt az elemet. A következő ütemezett mentés újra helyet csinál.

Egy tartomány vagy a Backup Everything indítása kihagyja azokat az elemeket, amelyeket valamelyik korlát visszatart, és megnevezi őket a válaszában. Egyik korlát sem vonatkozik a webes felületre és az ütemezésre. Az óránkénti keret a memóriában van, így a BombVault újraindítása lenullázza.

## Bekapcsolás {#switch-on}

1. Nyisd meg a **Beállítások, Rendszer, MCP-kiszolgáló** részt, és kattints a kliensed gombjára. A listában nem szereplő kliens az **Egyéb kliens** gombon át csatlakozik.
2. A **Kulcs** alatt hagyd meg az **Új kulcs** lehetőséget és a javasolt nevet, ami a kliens neve, vagy írj be egy olyat, amely megmondja, hol használod a kulcsot, például „Claude Code a laptopon”. Kliensenként egy kulccsal egyet visszavonhatsz a többi érintése nélkül. A **Meglévő kulcs** egy korábban létrehozott kulcsot ad a kliensnek.
3. Kapcsold be a **Mentések indításának engedélyezése** kapcsolót annál a kulcsnál, amelynek mentéseket kell tudnia indítani; enélkül a kulcs csak olvasni tud. Később a kulcs csempéjén módosíthatod, és a változás az asszisztens következő kérésétől érvényes, újracsatlakozás nélkül.
4. Kattints a **Kulcs létrehozása** gombra. A kulcs egyszer jelenik meg. A BombVault csak egy ujjlenyomatot őriz belőle, és nem tudja újra megmutatni, ezért most másold ki. Ha bezárod a párbeszédablakot, mielőtt a kliens használta a kulcsot, a kártya tovább mutatja, amíg meg nem erősíted, hogy kimásoltad.

Bejelentkezési jelszó nélkül maga a webes felület is nyitva áll mindenki előtt a hálózatodon, és aki meg tudja nyitni, kulcsot is létrehozhat. A kártya ezt jelzi. Ha a BombVaultot nyilvánosnak tűnő néven nyitod meg (például `bombvault.example.com` egy fordított proxy mögött), és nincs bejelentkezési jelszó, arról a címről nem lehet kulcsot létrehozni vagy cserélni, így egyetlen internetes weboldal sem veheti rá a böngésződet, hogy létrehozzon egyet. Állíts be bejelentkezési jelszót, vagy nyisd meg a BombVaultot az IP-címén vagy egy helyi néven, például `tower` vagy `tower.local`.

## A kulcsaid és a naplójuk {#keys}

Minden kulcsnak saját csempéje van a kártyán. Mutatja a kulcs nevét, hogy indíthat-e mentést vagy csak olvas, a kulcs utolsó négy karakterét, mikor hozták létre vagy cserélték utoljára, mikor használta utoljára egy kliens, és hány hívást tett ma. A csempén átnevezed a kulcsot, módosítod a jogosultságát, lecseréled vagy visszavonod. A visszavont kulcs a visszavont kulcsok listájába kerül, ahol végleg törölheted, amint az előzményekben egyetlen futás sem hivatkozik rá.

A csempe a neve mellett annak a kliensnek a jelét mutatja, amelyhez a kulcs készült. Az **Egyéb kliens** gombon át, vagy a kliensek listája előtt készült kulcsnál helyette egy kulcs látszik.

A csempe **Napló** gombja megnyitja, mit csinált a kulcs. Elöl az általa indított mentések állnak, mindegyik az állapotával és egy hivatkozással a futásra az irányítópult tevékenységnaplójában. Alattuk a hívásai, a legújabb elöl, az eszközzel és a hívás kimenetelével. Az elutasítás megmondja az okát: a kulcs csak olvashat, a megőrzésvédelem visszatartotta a mentést, már futott egy másik mentés, az elemet néhány perce mentették MCP-n keresztül, vagy a kulcs túl sok kérést küldött. A megszakítás arra a futásra hivatkozik, amelyről szólt.

A BombVault kulcsonként legfeljebb 30 napig őrzi a bejegyzéseket: a legújabb 500 sikeres indítást és megszakítást, mellettük pedig a legújabb 200 egyéb hívást (olvasások, elutasítások és hibák), így egy futó mentést újra és újra lekérdező vagy egy elutasított hívással újra és újra próbálkozó asszisztens nem tudja kiszorítani a naplóból a mentés indítását. Minden hívásnál az eszközt, a kimenetelt és a megszakítás által megnevezett futást tárolja. Soha nem tárolja, amit az asszisztens küldött, sem a kulcsot vagy az ujjlenyomatát. A diagnosztikai csomag csak megszámolja a bejegyzéseket, a beállítások exportja pedig kihagyja őket.

## Kliens csatlakoztatása {#clients}

Minden kliensnek van egy gombja a kártyán, az **Ezen a számítógépen** vagy **A felhőben** csoportban. A gomb három lépésből álló párbeszédablakot nyit: a kulcs; a kliens konfigurációja azzal a címmel, amelyen a kártyát megnyitottad, egy másológombbal, a konfiguráció helyével és a BombVault saját tanúsítványánál azzal, ami a kliensnek kell ahhoz, hogy megbízzon benne; végül a kliens első hívásának várása. A párbeszédablak figyeli a kulcs utolsó használatát, és zöldre vált, amikor a hívás megérkezik.

A párbeszédablak minden parancssortól távol tartja a kulcsot. Ahol a kliens környezeti változóból (`BOMBVAULT_MCP_KEY`), rejtett bekérésből vagy saját fájlból tudja olvasni, ott a konfiguráció csak megnevezi. Ahol a kliensnek nincs ilyen módja, a kulcs a konfigurációs fájljában vagy a beállításaiban van, és a párbeszédablak ezt ki is mondja. Ahol egy kliens dokumentációja nem mondja meg, hogyan bánik egy ismeretlen tanúsítvánnyal, ott a párbeszédablak ezt a lépést arra az esetre írja le, ha a kliens elutasítja a BombVault tanúsítványát.

| Kliens | Beállítás | Honnan jön a kulcs |
|---|---|---|
| AnythingLLM | konfigurációs fájl | a konfigurációs fájl |
| Antigravity | konfigurációs fájl | környezeti változó |
| Claude Code | parancs | kulcsfájl |
| Claude Desktop | konfigurációs fájl | kulcsfájl |
| Cline | konfigurációs fájl | a konfigurációs fájl |
| Codex CLI | konfigurációs fájl | környezeti változó |
| Continue | konfigurációs fájl | `~/.continue/.env` |
| Copilot CLI | konfigurációs fájl | a konfigurációs fájl |
| Cursor | konfigurációs fájl | környezeti változó |
| Gemini CLI | konfigurációs fájl | környezeti változó |
| GitHub Copilot (VS Code) | konfigurációs fájl | rejtett bekérés |
| Goose | konfigurációs fájl | környezeti változó |
| Jan | űrlap az alkalmazásban | az alkalmazás beállításai |
| JetBrains (AI Assistant, Junie) | konfigurációs fájl | a konfigurációs fájl |
| Kimi Code | konfigurációs fájl | a konfigurációs fájl |
| LM Studio | konfigurációs fájl | a konfigurációs fájl |
| Mistral Vibe | konfigurációs fájl | környezeti változó |
| Msty | űrlap az alkalmazásban | az alkalmazás beállításai |
| n8n | űrlap az alkalmazásban | az n8n hitelesítő adatai |
| Open WebUI | űrlap az alkalmazásban | az alkalmazás beállításai |
| opencode | konfigurációs fájl | környezeti változó |
| Perplexity (Mac) | űrlap az alkalmazásban | kulcsfájl |
| Qwen Code | konfigurációs fájl | környezeti változó |
| Roo Code | konfigurációs fájl | környezeti változó |
| Visual Studio | konfigurációs fájl | a konfigurációs fájl |
| Warp | konfigurációs fájl | a konfigurációs fájl |
| Windsurf | konfigurációs fájl | környezeti változó |
| Zed | konfigurációs fájl | a konfigurációs fájl |
| Grok | űrlap, a felhőben | a szolgáltató szerverei |
| Le Chat | űrlap, a felhőben | a szolgáltató szerverei |
| ChatGPT | OAuth-bejelentkezés, a felhőben | hozzáférési token, lásd [lent](#oauth) |
| Claude (claude.ai) | OAuth-bejelentkezés, a felhőben | hozzáférési token, lásd [lent](#oauth) |

Az alábbi szakaszok részletesebben leírják a Claude Code és a Claude Desktop beállítását, és felsorolják, mire van szüksége bármely más kliensnek.

### Claude Code {#claude-code}

A Claude Code az `mcp-remote` programon keresztül éri el a BombVaultot, amelyhez Node.js kell azon a számítógépen. Először mentsd el a kulcsot egy külön szövegfájlba, egyetlen sorként:

```text
X-API-Key: <your key>
```

Ezután futtasd le egyszer a kártya parancsát egy terminálban, a fájl elérési útjával kitöltve. Olyan tanúsítvány mögött, amelyben a számítógéped megbízik, így néz ki:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

A BombVault saját tanúsítványával (lásd [TLS és tanúsítványok](#tls)) a parancs a Node.js-t is a letöltött tanúsítványra irányítja:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

A kapcsolatot a Claude Code-ban a `/mcp` paranccsal ellenőrizheted. A `--scope user` az összes projektedben elérhetővé teszi a BombVaultot. A Claude Code csak a kulcsfájl elérési útját tárolja, így a kulcs nem jelenik meg sem a parancsban és a shell előzményeiben, sem a folyamatlistában. A fájlt olyan helyen tartsd, ahol csak te olvashatod, és minden olyan mappán kívül, amelyet commitolsz. Az `@latest` miatt az `npx` friss `mcp-remote` programot tölt le; különben egy régebbi, globálisan telepített változatot használna, amely nem ismeri a `--header-file` kapcsolót.

Ne írd a `${BOMBVAULT_MCP_KEY}` hivatkozást a Claude Code `mcp-remote` argumentumai közé. A Claude Code az ilyen hivatkozást a saját környezetéből tölti ki, mielőtt elindítja az `mcp-remote` programot, így a kulcs annak a folyamatnak a parancssorába kerül, ahol a számítógép más programjai és felhasználói is olvashatják.

Node.js nélkül, és csak olyan tanúsítvány mögött, amelyben a számítógéped megbízik, a Claude Code önállóan is tud kapcsolódni. Tegyél egy `.mcp.json` fájlt a projekt mappájába:

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

Állítsd be a `BOMBVAULT_MCP_KEY` változót ott, ahol a Claude Code indul, például a `~/.claude/settings.json` fájl `"env"` részében vagy a shell profiljában, szövegszerkesztővel és nem a parancssorba gépelve. Itt a hivatkozás biztonságos, mert a Claude Code nem indít második folyamatot, amelybe a kulcs bekerülne. A BombVault saját tanúsítványával ez nem működik: a Claude Code saját kapcsolata beállított `NODE_EXTRA_CA_CERTS` mellett is elutasítja. Soha ne commitolj olyan `.mcp.json` fájlt, amelybe a kulcs bele van írva.

### Claude Desktop {#claude-desktop}

A Claude Desktop az `mcp-remote` programon keresztül éri el a BombVaultot, amelyhez Node.js kell azon a számítógépen. Először mentsd a kulcsot egy külön szövegfájlba, egyetlen sorként, ahogy a [Claude Code](#claude-code) résznél áll. Nyisd meg a konfigurációs fájlt a Claude Desktopban a **Settings, Developer, Edit Config** útvonalon. Windowson a `%APPDATA%\Claude\claude_desktop_config.json`, macOS-en a `~/Library/Application Support/Claude/claude_desktop_config.json` helyen van. Add hozzá a kártya bejegyzését a `"mcpServers"` részen belül, a már ott lévő kiszolgálók mellé, és indítsd újra a Claude Desktopot:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- A `NODE_EXTRA_CA_CERTS` csak a BombVault saját tanúsítványa miatt van ott. Olyan tanúsítvány mögött, amelyben a számítógéped már megbízik, hagyd el.
- A `--allow-http` csak egyszerű `http://` címnél kerül bele.
- Windowson normál perjellel írd az útvonalakat, például `C:/Users/sam/bombvault-key.txt`, mert egy magában álló fordított perjel nem érvényes JSON. A kulcsfájl útvonalában ne legyen szóköz: a Claude Desktop Windowson a szóközt tartalmazó útvonalat két darabban adja át az `npx`-nek.
- A konfiguráció csak a kulcsfájlt nevezi meg, így a kulcs sem benne, sem a folyamatlistában nem jelenik meg. A fájlt olyan helyen tartsd, ahol csak te olvashatod.

### Kliensek a felhőben {#cloud-clients}

A ChatGPT, a claude.ai-os Claude, a Grok és a Le Chat a szolgáltatójuk szervereiről hívják a BombVaultot, ezért a BombVaultnak az internetről elérhetőnek kell lennie nyilvánosan megbízható tanúsítvánnyal, például fordított proxy mögött; a Le Chat elutasítja az önaláírtakat. A proxyn lévő bejelentkezés védheti a webes felületet, de a `/mcp` útvonalnak nélküle kell eljutnia a BombVaultig: ezek a szolgáltatások nem tudnak bejelentkezni egy proxyba, és a BombVault maga ellenőrzi a kulcsukat vagy tokenjüket. A Grok és a Le Chat rögzített kulcsot küld, és a gombjaik ugyanúgy állítják be őket, mint a többit. A ChatGPT, és a legtöbb szervezetben a claude.ai-os Claude is, csak OAuth-bejelentkezéssel csatlakozik, amelyet alább írunk le.

### OAuth-bejelentkezés {#oauth}

Az olyan kliens számára, amely nem fogad el kulcsot, a BombVault a saját OAuth-engedélyezési kiszolgálója. A kliens magától regisztrál, egy BombVault-oldalra küld, ahol a belépési jelszavaddal (és a második faktorral, ha beállítottad) bejelentkezel és engedélyezed. A kliens ezután olyan tokent kap, amely csak ennek a BombVaultnak az MCP-végpontjához érvényes, és magától megújítja.

1. Állíts be belépési jelszót a **Beállítások, Rendszer** alatt. Enélkül a BombVault egyáltalán nem kínál bejelentkezést, mert nem volna kitől hozzájárulást kérni.
2. Tedd a BombVaultot az internetről https-en elérhetővé olyan tanúsítvánnyal, amelyben a böngészők megbíznak, általában fordított proxyn keresztül. A kliens a saját szervereiről hívja a `/mcp`, `/oauth/` és `/.well-known/` útvonalakat, ezért egy saját bejelentkezéssel rendelkező proxynak ezt a hármat át kell engednie a BombVaultig. A `/oauth/authorize` alatti hozzájárulási oldal a saját böngésződben nyílik meg, és maradhat a proxy bejelentkezése mögött. Add meg a proxyt a `TRUSTED_PROXY` változóban is (lásd [Beállítások](configuration.md)). A BombVault címenként korlátozza a kliensek regisztrációját, és enélkül minden kliens úgy tűnik, mintha a proxytól jönne.
3. Az MCP-kártyán kapcsold be az **OAuth-bejelentkezés** kapcsolót, és add meg a **Nyilvános cím** mezőt: a https-címet útvonal nélkül, például `https://backup.example.com`. Minden token ehhez a címhez kötődik, így módosítás után minden kliensnek újra be kell jelentkeznie.
4. Kattints a ChatGPT vagy a Claude gombjára. A párbeszédablak mutatja a **Csatoló URL-je** értéket, vagyis a nyilvános címet `/mcp` végződéssel, és hogy hová kerül az adott kliensben. A ChatGPT-ben a **Beállítások, Alkalmazások és csatolók, Speciális beállítások** alatt bekapcsolod a fejlesztői módot, a **Létrehozás** lehetőséget választod, MCP-szerver URL-ként beilleszted a csatoló URL-jét, és hitelesítésnek az OAuth-ot választod. A claude.ai-on megnyitod a **Beállítások, Csatolók, Egyéni csatoló hozzáadása** részt, beilleszted a csatoló URL-jét, az OAuth kliensazonosítót és a titkot üresen hagyod, és a **Csatlakozás** lehetőséget választod.
5. A kliens megnyitja a hozzájárulási oldalt. Ez mutatja, ki kér, hová küld vissza a válaszod, és a **Mentések indításának engedélyezése** kapcsolót, amely kezdetben ki van kapcsolva. Válaszd az **Engedélyezés** vagy az **Elutasítás** lehetőséget.

Minden bejelentkezett kliens csempét kap a kulcsok mellett a jelével, a naplójával, a **Visszavonás** gombbal és a **Mentések indításának engedélyezése** kapcsolóval, és ugyanazok a korlátok vonatkoznak rá, mint egy kulcsra. A visszavonás azonnal hat. Ha ugyanaz a kliens újra bejelentkezik, az új engedélye felváltja a régit, és az az engedély, amelyet 30 napig senki nem használt, lejár. Egyszerre legfeljebb 10 kliens lehet bejelentkezve, a 10 kulcson felül.

A hozzájárulási oldal csak olyan regisztrált klienstől fogad el kérést, amely pontosan az egyik regisztrált visszatérési címét adja meg: https, vagy tetszőleges portú loopback-cím a saját gépeden futó kliens esetén. Csak a PKCE-vel (S256) védett engedélyezési kódos folyamatot fogadja el, és a válaszod a munkamenetedhez kötődik, így más weboldal nem küldheti el helyetted. A hozzáférési tokenek egy óráig érvényesek. A frissítési tokent minden használatkor lecseréli, és ha utána újra felbukkan egy, a BombVault visszavonja az engedélyt, mert valaki másnál van egy másolata. Az a kliens, amely 30 másodpercen belül megismétli a legutóbbi frissítését, mert a válasz nem ért el hozzá, ehelyett új tokeneket kap. A BombVault nem tölt le klienseknek szóló metaadatokat az internetről, ezért a kliensek dinamikus klienregisztrációval regisztrálnak.

### Más kliensek {#other-clients}

Bármelyik kliens megfelel, amely Streamable HTTP-t beszél:

- URL: a webes felület címe és utána `/mcp`, például `https://192.168.1.10:3443/mcp`.
- A kulcs az `Authorization: Bearer <key>` vagy az `X-API-Key: <key>` fejlécben. Ha mindkettő jön, ugyanazt a kulcsot kell tartalmazniuk.
- `POST` kérés `Content-Type: application/json` és `Accept: application/json, text/event-stream` fejlécekkel.
- Kérésenként egy JSON-RPC üzenet; a kötegeket (batch) elutasítja.
- Protokollverziók: 2026-07-28, 2025-11-25, 2025-06-18 és 2025-03-26.

## TLS és tanúsítványok {#tls}

A BombVault saját maga által kiállított tanúsítvánnyal szolgálja ki a HTTPS-t, és ez a tanúsítvány kezdetben csak a `localhost`, `127.0.0.1` és `::1` neveket tartalmazza. A Claude Code és az `mcp-remote` helyi hálózati címen elutasítja. A megoldások abban a sorrendben, amely a legtöbb Unraid-telepítéshez illik:

1. **Add hozzá a címet az MCP-kártyán.** Ha a kártyát HTTPS-en olyan címen nyitod meg, amelyet a tanúsítvány nem tartalmaz, a kártya jelzi ezt, és felkínálja a **Cím felvétele a tanúsítványba** gombot. A BombVault ekkor újra kiállítja a tanúsítványát ezzel a címmel (a böngésződ még egyszer figyelmeztet, ahogy először). Ezután kattints a **Tanúsítvány letöltése** gombra; a részletek a `NODE_EXTRA_CA_CERTS` értékét a letöltött fájlra állítják, így a kliens pontosan ebben a tanúsítványban bízik meg. Ez azt is jelenti, hogy minden korábban letöltött fájllal beállított kliens nem csatlakozik többé, amint a tanúsítványt újra kiállítják, sem ezen a gépen, sem bármelyik másikon, amíg meg nem kapja az új fájlt.
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

- Aktív kulcs nélkül és kikapcsolt OAuth-bejelentkezésnél a `/mcp` `404`-et válaszol.
- Az OAuth-bejelentkezés csak addig érhető el, amíg belépési jelszó van beállítva. A tokeneket, kódokat és klienstitkokat csak ujjlenyomatként tárolja, és egy token csak ahhoz a címhez érvényes, amelyhez kiállították.
- Egy kliens egy címről óránként legfeljebb 10-szer regisztrálhat, és a BombVault legfeljebb 100 olyan regisztrált klienst tart meg, amellyel senki nem jelentkezett be, mindegyiket egy napig. A hibás kódok és frissítési tokenek ugyanabba a zárolásba számítanak, mint a hibás kulcsok.
- Az engedélyek a konfigurációs mentés visszaállításakor vagy az `APP_KEY` megváltozásakor úgy viselkednek, mint a kulcsok: visszaállítás után minden kliensnek újra be kell jelentkeznie.
- Nincs kivételezett cím. A `localhost`, az Unraid-gazdagép, egy fordított proxy vagy a `tailscale serve` felől érkező kérésekhez ugyanúgy kulcs kell, mint bármely máshoz, akkor is, ha a webes felületnek nincs bejelentkezési jelszava.
- A kulcsokat csak ujjlenyomatként tárolja, egyszer jeleníti meg, és átnevezhetők, cserélhetők, visszavonhatók. Legfeljebb 10 aktív kulcs, mindegyik saját **Mentések indításának engedélyezése** kapcsolóval.
- Minden létrehozás, csere, jogosultság-módosítás és visszavonás értesítést küld az értesítési csatornáidon a címmel együtt, ahonnan érkezett, hacsak nincsenek kikapcsolva az értesítések.
- Címenként percenként 5 hibás kulcs, utána `429`. Kulcsonként percenként 120 kérés és óránként 12 elindított mentés, ehhez jön a fent leírt várakozási idő és a megőrzésvédelem.
- Egy másik originről érkező böngészőoldal kéréseit elutasítja.
- Amíg nincs bejelentkezési jelszó, nyilvánosnak tűnő gazdagépnévről nem lehet kulcsot létrehozni.
- Minden mentés, amelyet egy asszisztens indít, és az ebből következő prune és off-site futások "MCP-n keresztül" jelölést kapnak a kulcs nevével a tevékenységnaplóban, a hibapanelen és a mentés értesítésében.
- Minden eszközhívás bekerül a konténer naplójába a kulcs azonosítójával és utolsó négy karakterével (a nevével soha), és a `/metrics` számolja (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Egy konfigurációs mentés visszaállítása minden kulcsot visszavon, mert a visszaállított adatbázis olyan kulcsokat is tartalmazhat, amelyeket a mentése után vontál vissza. Utána hozz létre új kulcsokat.
- Egy kulcs megszűnik működni, ha az `APP_KEY` megváltozik (újratelepítés vagy visszaállítás egy másik konténerbe). A kártya ezt észleli és megjelöli a kulcsot, a **Kulcs cseréje** pedig újra érvényes titkot ad neki.
- Kezeld a kulcsot jelszóként. Az a kliens, amely sem környezeti változóból, sem bekérésből, sem kulcsfájlból nem tudja olvasni a kulcsot, titkosítatlan szövegként tartja a konfigurációjában vagy a beállításaiban, és a párbeszédablaka ezt ki is mondja. Egy kevésbé megbízható számítógépen inkább csak olvasó kulcsot használj.

## Mi hagyja el a gépet {#privacy}

Mindaz, amit egy asszisztens elolvas, a mögötte álló MI-szolgáltatóhoz kerül: az elemek nevei, az ütemezések, a futási előzmények hibaüzenetekkel, a visszaállítási pontok azonosítói és időpontjai, az adatbázismotorok nevei és a dumpok mérete, a folyamatban lévő tevékenység, a tárolási adatok, a lefedettség és az állapot. A BombVault eltávolítja a gazdagép útvonalait, a tárolók helyét, a gazdagépneveket, a hitelesítő adatokat, a hook-parancsokat és a kulcsokat, mielőtt bármi kimenne.

## Hibaelhárítás {#troubleshooting}

| Amit látsz | Amit jelent |
|---|---|
| `404` | Nincs aktív kulcs, és az OAuth-bejelentkezés ki van kapcsolva, vagy rossz az útvonal, például `/api/mcp`. A végpont a `/mcp`. |
| `401` | A kulcs hiányzik, elgépelték, visszavonták vagy lecserélték. Lehet, hogy egy proxy eldobja az `Authorization` fejlécet (próbáld az `X-API-Key` fejlécet). Ha a kártya a kulcsot már nem érvényesnek jelöli, megváltozott az `APP_KEY`: cseréld le a kulcsot. |
| `403` | A kérés egy másik originről érkező böngészőoldalról jött. Használj asztali vagy parancssori klienst. |
| `405` GET esetén | Normális. A végpont csak `POST` kérést fogad. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | A kliens túl régi a Streamable HTTP-hez. Frissítsd. |
| `400` "batch requests are not accepted" | A kliens JSON-RPC kötegeket küld. Kérésenként egy üzenetet küldj. |
| `429` | Túl sok hibás kulcs erről a címről, vagy egy kulccsal több mint 120 kérés percenként. Várj egy percet, és nézd meg, nem ragadt-e az asszisztens egy ciklusba. |
| "certificate", "self-signed" vagy "unable to verify" szövegű hibák | A kliens nem bízik a BombVault tanúsítványában. Lásd [TLS és tanúsítványok](#tls). |
| `busy` | Egy másik mentés vagy karbantartási feladat foglalja a tartományt. Próbáld újra, ha végzett. |
| `cooldown` | Ezt az elemet, ezt a tartományt vagy a Backup Everythinget kevesebb mint 15 perce indították MCP-n keresztül. |
| `retention_guard` | Még egy MCP-mentés után "az utolsó N megtartása" ablakban csak MCP-ből származó visszaállítási pontok maradnának, vagy az elem az elmúlt 24 órában már 4 mentést kapott MCP-n keresztül, a sikertelenekkel és a megszakítottakkal együtt. Az első esetben a következő ütemezett mentés helyet csinál, a másodikban az elem a legrégebbi ilyen mentés után 24 órával lesz újra szabad. A webes felületen bármikor elindíthatod. |
| `rate_limited` | A kulcs elhasználta az erre az órára jutó 12 indítását. |
| `not_permitted` indításnál | A kulcs csak olvashat. Kapcsold be az **Mentések indításának engedélyezése** kapcsolót a kártyán; újracsatlakozás nem kell. Megszakításnál azt jelenti, hogy a futást nem ez a kulcs indította. |
| `domain_off` | Ez a mentésfajta ki van kapcsolva a beállításokban. |
| `not_found` | A BombVault nem védi ezt az elemet. Előbb vedd fel a webes felületen; az MCP soha nem hoz létre beállítást. |
| A kliens nem találja az engedélyezési kiszolgálót | Az OAuth-bejelentkezés ki van kapcsolva, nincs belépési jelszó, vagy a proxy nem engedi át a `/.well-known/` útvonalat a BombVaultig. |
| A hozzájárulási oldal szerint a visszatérési cím nincs regisztrálva | A kliens olyan visszatérési címet küldött, amelyet nem regisztrált. Távolítsd el a csatolót a kliensben, és add hozzá újra. |
| Egy bejelentkezett kliens `401`-et kap | Az engedélyét visszavonták, 30 nap használaton kívül után lejárt, vagy megváltozott a nyilvános cím. A kliens újra bejelentkezik. |

Ne állítsd be a konténeren az `MCPGODEBUG` környezeti változót. Megváltoztatja az MCP-könyvtár működését, és egy hibás érték indításkor leállítja a BombVaultot, mielőtt egyetlen naplósort is írna.
