# Server MCP

BombVault are un server integrat pentru Model Context Protocol (MCP), protocolul prin care asistenții AI precum Claude Code și Claude Desktop ajung la unelte externe. Prin el, un asistent poate citi cum stau copiile tale și, dacă îi permiți, poate porni o copie sau poate anula una pornită de el. Este oprit până când creezi o cheie sau activezi [autentificarea prin OAuth](#oauth): până atunci, endpoint-ul `/mcp` răspunde `404` la orice.

## Ce poate și ce nu poate face un asistent {#tools}

| Unealtă | Ce face | Tip |
|---|---|---|
| `get_health` | Versiunea, numele instanței, dacă rulează o copie și ce are voie să facă această cheie | citire |
| `get_status` | Starea protecției pe fiecare domeniu: ultima copie reușită, intervalul așteptat, verificările și controalele off-site, următoarele rulări programate, iar pentru containere cel mai recent test de pornire | citire |
| `get_coverage` | Ce protejează BombVault și ce nu, cu motivul pentru fiecare | citire |
| `list_items` | Fiecare container, VM și set de foldere protejat, stick-ul flash și configurația aplicației, cu programarea, ce oprește o copie, ultima copie și cât a durat; containerele de baze de date arată și ultimul dump; apar și seturile de date ZFS, cu rezultatul ultimei lor verificări; fiecare element are ultima sa verificare a restaurării, iar un container și ultimul test de pornire sau motivul pentru care nu poate fi testat; un container recreat cu alte setări de la ultima copie arată ce s-a schimbat | citire |
| `list_runs` | Istoricul rulărilor, cele mai noi primele, filtrabil după domeniu, element, stare, tip și timp; o copie lentă frânată de un singur lucru îl numește | citire |
| `list_restore_points` | Punctele de restaurare ale unui element din depozitul lui principal și, pentru un container, dumpurile lui de baze de date; un set de date ZFS are câte un punct de restaurare pentru fiecare copie, cu un snapshot al fiecărui set de date de sub el | citire |
| `get_activity` | Ce rulează chiar acum, cu fază și procent | citire |
| `get_storage_stats` | Istoricul de dimensiune al depozitului principal al unui domeniu și creșterea lui pe săptămână, plus spațiul folosit, liber și total pe discul sau locația la distanță a fiecăruia dintre depozitele sale | citire |
| `get_size_breakdown` | Ce dosare și fișiere ocupă spațiu în cea mai nouă copie a unui container, a unei VM sau a unui set de dosare, și cât din ele a adăugat ultima copie | citire |
| `list_anomalies` | Anomaliile observate de BombVault în copii, filtrabile după stare, gravitate și domeniu, cu un rezumat al celor deschise | citire |
| `get_anomaly` | Una dintre aceste constatări, cu nota lăsată la confirmarea ei | citire |
| `start_backup` | Face imediat copia unui element | pornire |
| `start_domain_backup` | Face copia fiecărui element protejat dintr-un domeniu | pornire |
| `start_backup_everything` | Rulează trecerea Backup Everything | pornire |
| `cancel_backup` | Anulează o copie în curs pornită de această cheie | anulare |

Rămân în interfața web: restaurările de orice fel (inclusiv descărcarea, salvarea sau importul unui dump de bază de date), ștergerea copiilor, prune, unlock, verificările și exercițiile, replicarea off-site, setările, datele de autentificare și cheile MCP, precum și anularea unei copii pornite de programare, de interfața web sau de altă cheie. La fel și confirmarea unei anomalii sau marcarea ei ca așteptată, care se face pe pagina **Anomalii**. Motivul: răspunsurile uneltelor conțin nume și mesaje de eroare de pe serverul tău, iar oricare dintre ele poate conține un text scris ca să manipuleze asistentul. Un asistent care se lasă păcălit de un asemenea text poate cel mult să pornească o copie în limitele de mai jos sau să anuleze una pe care a pornit-o el.

Dacă depozitul principal al unui element e la distanță (S3, REST, SFTP, rclone), `list_restore_points` îl contactează, iar apelul poate dura puțin. Copiile off-site nu pot fi listate prin MCP. Ce urmăresc verificările de anomalii se descrie în [Funcționalități](features.md), iar cum păstrează un element ZFS câte un snapshot pentru fiecare set de date, în [Seturi de date ZFS](zfs-datasets.md#contents).

## Ce face o copie pornită {#starting-backups}

Copia unui asistent e aceeași copie pe care o pornește interfața web. Un container care rulează este oprit până se termină copia lui, împreună cu containerele setate să se oprească odată cu el. O VM cu metoda "graceful" este oprită și pornită din nou. Un set de date ZFS oprește containerele setate pentru el cât timp i se face snapshotul. Seturile de foldere, stick-ul flash și configurația continuă să ruleze. După aceea, BombVault aplică politica de păstrare și poate copia în depozitul off-site. `list_items` îi spune asistentului ce oprește un element și cât a durat ultima lui copie, iar descrierile uneltelor îi cer să-ți spună asta înainte să pornească ceva.

Pentru că o copie oprește servicii și scoate afară puncte de restaurare vechi, pornirile prin MCP sunt limitate:

- 12 copii pornite pe oră pentru fiecare cheie.
- 15 minute între două porniri MCP ale aceluiași element, aceluiași domeniu sau Backup Everything.
- Cel mult 4 porniri MCP ale aceluiași element în 24 de ore.
- **Protecția păstrării.** Când un domeniu păstrează un număr fix de puncte de restaurare (doar "păstrează ultimele N", fără regulă zilnică, săptămânală sau lunară, local sau pe o destinație off-site), fiecare copie nouă scoate afară cea mai veche. BombVault refuză atunci o pornire MCP a unui element ale cărui cele mai noi N-1 copii reușite au fost pornite toate prin MCP. Astfel, în setul păstrat rămâne mereu cel puțin un punct de restaurare făcut de programare sau de tine. Cu "păstrează ultima 1", un asistent nu poate face deloc copia acelui element. Următoarea copie programată face din nou loc. O regulă anuală singură contează ca "păstrează ultima 1", pentru că păstrează un singur punct de restaurare pentru anul curent.

O pornire de domeniu sau de Backup Everything lasă pe dinafară elementele reținute de o limită și le numește în răspuns. Interfața web și programarea nu sunt atinse de niciuna dintre aceste limite. Bugetul orar stă în memorie, așa că o repornire a BombVault îl readuce la zero.

## Pornire {#switch-on}

1. Deschide **Setări, Integrări, Server MCP** și dă clic pe butonul clientului tău. Un client care nu e în listă se conectează prin **Alt client**.
2. La **Cheie** lasă **Cheie nouă** și numele propus, adică al clientului, sau scrie unul care spune unde e folosită cheia, de exemplu „Claude Code pe laptop”. O cheie pentru fiecare client îți permite să revoci una fără să le atingi pe celelalte. **Cheie existentă** îi dă clientului o cheie creată mai devreme.
3. Pornește **Permite pornirea copiilor** pentru o cheie care trebuie să poată porni copii; fără asta poate doar să citească. Poți schimba asta mai târziu pe placa cheii, iar schimbarea se aplică de la următoarea cerere a asistentului, fără reconectare.
4. Dă clic pe **Creează cheia**. Cheia e afișată o singură dată. BombVault păstrează doar o amprentă a ei și nu o mai poate arăta, așa că copiaz-o acum. Dacă închizi dialogul înainte ca clientul să fi folosit cheia, cardul continuă să o afișeze până confirmi că ai copiat-o.

Fără parolă de autentificare, interfața web însăși e deschisă pentru toată lumea din rețeaua ta, iar cine o poate deschide poate crea și o cheie. Cardul spune asta. Dacă deschizi BombVault sub un nume care pare public (de exemplu `bombvault.example.com` în spatele unui proxy invers) și nu e setată nicio parolă de autentificare, de la acea adresă nu se pot crea și nici înlocui chei, ca nicio pagină web de pe internet să nu-ți poată face browserul să creeze una. Setează o parolă de autentificare sau deschide BombVault prin adresa IP ori printr-un nume local precum `tower` sau `tower.local`.

## Cheile tale și jurnalul lor {#keys}

Fiecare cheie are propria dală pe card. Arată numele cheii, dacă poate porni copii sau doar citește, ultimele patru caractere ale cheii, când a fost creată sau înlocuită ultima dată, când a folosit-o ultima dată un client și câte apeluri a făcut azi. Pe dală redenumești cheia, îi schimbi permisiunea, o înlocuiești sau o revoci. O cheie revocată trece în lista cheilor revocate, unde o poți șterge definitiv când nicio rulare din istoric nu o mai numește.

Lângă nume, placa arată sigla clientului pentru care a fost creată cheia. O cheie creată prin **Alt client**, sau înainte ca cardul să listeze clienți, arată în schimb o cheie.

**Jurnal** pe o dală deschide ce a făcut acea cheie. Întâi vin copiile pe care le-a pornit, fiecare cu starea ei și un link către acea rulare în jurnalul de activitate de pe tabloul de bord. Dedesubt sunt apelurile ei, cele mai noi primele, cu instrumentul și rezultatul apelului. Un refuz spune de ce: cheia poate doar citi, protecția de păstrare a oprit copia, rula deja altă copie, elementul a fost copiat prin MCP acum câteva minute sau cheia a trimis prea multe cereri. O anulare trimite la rularea la care se referea.

BombVault păstrează intrările fiecărei chei cel mult 30 de zile: cele mai noi 500 de porniri și anulări reușite și, alături de ele, cele mai noi 200 de alte apeluri (citiri, refuzuri și erori), astfel încât un asistent care interoghează mereu o copie în curs sau reîncearcă mereu un apel refuzat nu poate împinge pornirea ei afară din jurnal. Pentru fiecare apel salvează instrumentul, rezultatul și rularea numită de o anulare. Nu salvează niciodată ce a trimis asistentul, nici cheia sau amprenta ei. Pachetul de diagnostic doar numără intrările, iar un export al setărilor le lasă deoparte.

## Conectarea unui client {#clients}

Fiecare client are un buton pe card, sub **Pe acest computer** sau **În cloud**. Butonul deschide un dialog în trei pași: cheia; configurația pentru acel client, cu adresa la care ai deschis cardul, un buton pentru copiere, locul unde stă configurația și, cu certificatul propriu al BombVault, ce îi trebuie clientului ca să aibă încredere în el; și așteptarea primului apel al clientului. Dialogul urmărește ultima folosire a cheii și devine verde când apelul sosește.

Dialogul ține cheia departe de orice linie de comandă. Acolo unde clientul o poate citi dintr-o variabilă de mediu (`BOMBVAULT_MCP_KEY`), dintr-o solicitare mascată sau dintr-un fișier propriu, configurația doar o numește. Acolo unde clientul nu are o asemenea cale, cheia stă în fișierul lui de configurare sau în setări, iar dialogul spune asta. Acolo unde documentația unui client nu spune cum tratează un certificat necunoscut, dialogul scrie pasul ca ce e de făcut dacă clientul respinge certificatul BombVault.

| Client | Configurare | De unde vine cheia |
|---|---|---|
| AnythingLLM | fișier de configurare | fișierul de configurare |
| Antigravity | fișier de configurare | variabilă de mediu |
| Claude Code | comandă | fișierul cheii |
| Claude Desktop | fișier de configurare | fișierul cheii |
| Cline | fișier de configurare | fișierul de configurare |
| Codex CLI | fișier de configurare | variabilă de mediu |
| Continue | fișier de configurare | `~/.continue/.env` |
| Copilot CLI | fișier de configurare | fișierul de configurare |
| Cursor | fișier de configurare | variabilă de mediu |
| Gemini CLI | fișier de configurare | variabilă de mediu |
| GitHub Copilot (VS Code) | fișier de configurare | solicitare mascată |
| Goose | fișier de configurare | variabilă de mediu |
| Jan | formular în aplicație | setările aplicației |
| JetBrains (AI Assistant, Junie) | fișier de configurare | fișierul de configurare |
| Kimi Code | fișier de configurare | fișierul de configurare |
| LM Studio | fișier de configurare | fișierul de configurare |
| Mistral Vibe | fișier de configurare | variabilă de mediu |
| Msty | formular în aplicație | setările aplicației |
| n8n | formular în aplicație | datele de autentificare din n8n |
| Open WebUI | formular în aplicație | setările aplicației |
| opencode | fișier de configurare | variabilă de mediu |
| Perplexity (Mac) | formular în aplicație | fișierul cheii |
| Qwen Code | fișier de configurare | variabilă de mediu |
| Roo Code | fișier de configurare | variabilă de mediu |
| Visual Studio | fișier de configurare | fișierul de configurare |
| Warp | fișier de configurare | fișierul de configurare |
| Windsurf | fișier de configurare | variabilă de mediu |
| Zed | fișier de configurare | fișierul de configurare |
| Grok | formular, în cloud | serverele furnizorului |
| Le Chat | formular, în cloud | serverele furnizorului |
| ChatGPT | autentificare prin OAuth, în cloud | un token de acces, vezi [mai jos](#oauth) |
| Claude (claude.ai) | autentificare prin OAuth, în cloud | un token de acces, vezi [mai jos](#oauth) |

Secțiunile de mai jos explică mai detaliat configurarea Claude Code și Claude Desktop și arată de ce are nevoie orice alt client.

### Claude Code {#claude-code}

Claude Code ajunge la BombVault prin `mcp-remote`, care are nevoie de Node.js pe acel calculator. Mai întâi salvează cheia într-un fișier text separat, pe un singur rând:

```text
X-API-Key: <your key>
```

Apoi rulează o dată comanda de pe card într-un terminal, cu calea acelui fișier completată. În spatele unui certificat în care calculatorul tău are încredere, arată așa:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Cu certificatul propriu al BombVault (vezi [TLS și certificate](#tls)), comanda îi arată în plus lui Node.js certificatul descărcat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Verifică legătura cu `/mcp` în Claude Code. `--scope user` face BombVault disponibil în toate proiectele tale. Claude Code păstrează doar calea fișierului cu cheia, așa că cheia nu apare nici în comandă și în istoricul shell-ului, nici în lista de procese. Ține fișierul într-un loc unde doar tu îl poți citi și în afara oricărui folder pe care îl pui în commit. `@latest` face ca `npx` să descarce un `mcp-remote` actual; altfel s-ar folosi o versiune mai veche instalată global, care nu cunoaște `--header-file`.

Nu scrie `${BOMBVAULT_MCP_KEY}` în argumentele `mcp-remote` pentru Claude Code. Claude Code completează o astfel de referință din propriul mediu înainte să pornească `mcp-remote`, așa că cheia ajunge în linia de comandă a acelui proces, unde alte programe și utilizatori ai calculatorului o pot citi.

Fără Node.js, și doar în spatele unui certificat în care calculatorul tău are încredere, Claude Code se poate conecta singur. Pune un `.mcp.json` în folderul proiectului:

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

Setează `BOMBVAULT_MCP_KEY` acolo unde pornește Claude Code, de exemplu sub `"env"` în `~/.claude/settings.json` sau în profilul shell-ului, editat într-un editor de text, nu tastat la prompt. Aici referința e sigură, pentru că Claude Code nu pornește un al doilea proces în care ar ajunge cheia. Cu certificatul propriu al BombVault asta nu merge: conexiunea proprie a lui Claude Code îl refuză chiar și cu `NODE_EXTRA_CA_CERTS` setat. Nu face niciodată commit unui `.mcp.json` în care cheia e scrisă direct.

### Claude Desktop {#claude-desktop}

Claude Desktop ajunge la BombVault prin `mcp-remote`, care are nevoie de Node.js pe acel calculator. Salvează mai întâi cheia într-un fișier text separat, pe o singură linie, cum e descris la [Claude Code](#claude-code). Deschide fișierul de configurare în Claude Desktop din **Settings, Developer, Edit Config**. Se află în `%APPDATA%\Claude\claude_desktop_config.json` pe Windows și în `~/Library/Application Support/Claude/claude_desktop_config.json` pe macOS. Adaugă intrarea de pe card în `"mcpServers"`, lângă serverele care sunt deja acolo, și repornește Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` e acolo doar pentru certificatul propriu al BombVault. În spatele unui certificat în care calculatorul tău are deja încredere, scoate-l.
- `--allow-http` se adaugă doar pentru o adresă `http://` simplă.
- Pe Windows, scrie căile cu bare oblice normale, de exemplu `C:/Users/sam/bombvault-key.txt`, pentru că o singură bară oblică inversă nu este JSON valid. Păstrează calea fișierului cu cheia fără spații: Claude Desktop pe Windows îi transmite lui `npx` o cale cu spațiu în două bucăți.
- Configurația numește doar fișierul cu cheia, așa că cheia nu apare nici în ea, nici în lista de procese. Ține fișierul într-un loc unde doar tu îl poți citi.

### Clienți în cloud {#cloud-clients}

ChatGPT, Claude pe claude.ai, Grok și Le Chat apelează BombVault de pe serverele furnizorului lor, deci BombVault trebuie să fie accesibil din internet cu un certificat de încredere publică, de exemplu în spatele unui proxy invers; Le Chat refuză certificatele autosemnate. O autentificare pe proxy poate proteja interfața web, dar `/mcp` trebuie să ajungă la BombVault fără ea: aceste servicii nu se pot autentifica la un proxy, iar BombVault le verifică singur cheia sau tokenul. Grok și Le Chat trimit o cheie fixă, iar butoanele lor le configurează ca pe celelalte. ChatGPT, și în majoritatea organizațiilor și Claude pe claude.ai, se conectează doar prin autentificare cu OAuth, descrisă mai jos.

### Autentificare prin OAuth {#oauth}

Pentru un client care nu poate primi o cheie, BombVault este propriul lui server de autorizare OAuth. Clientul se înregistrează singur, te trimite pe o pagină BombVault, iar acolo te autentifici cu parola de conectare (și cu al doilea factor, dacă l-ai configurat) și îi dai permisiunea. Clientul primește apoi un token care funcționează doar pentru endpoint-ul MCP al acestui BombVault și îl reînnoiește singur.

1. Setează o parolă de conectare la **Setări, Securitate**. Fără ea, BombVault nu oferă nicio autentificare, pentru că nu ar exista cineva căruia să i se ceară acordul.
2. Fă BombVault accesibil din internet prin https, cu un certificat în care browserele au încredere, de obicei printr-un proxy invers. Clientul apelează `/mcp`, `/oauth/` și `/.well-known/` de pe propriile servere, așa că un proxy cu propria autentificare trebuie să lase aceste trei căi să treacă până la BombVault. Pagina de acord de la `/oauth/authorize` se deschide în propriul tău browser și poate rămâne în spatele autentificării proxy-ului. Trece proxy-ul și în `TRUSTED_PROXY` (vezi [Configurare](configuration.md)). BombVault limitează înregistrările clienților pe adresă, iar fără asta fiecare client pare să vină de la proxy.
3. Pe cardul MCP, activează **Autentificare prin OAuth** și introdu **Adresă publică**: adresa https fără cale, de exemplu `https://backup.example.com`. Fiecare token este legat de această adresă, așa că după o schimbare fiecare client trebuie să se autentifice din nou.
4. Apasă butonul ChatGPT sau Claude. Dialogul arată **URL-ul conectorului**, adică adresa publică urmată de `/mcp`, și unde se pune în acel client. În ChatGPT activezi modul dezvoltator la **Setări, Aplicații și conectori, Setări avansate**, alegi **Creează**, lipești URL-ul conectorului ca URL al serverului MCP și alegi OAuth ca autentificare. Pe claude.ai deschizi **Setări, Conectori, Adaugă conector personalizat**, lipești URL-ul conectorului, lași goale ID-ul de client și secretul OAuth și alegi **Conectează**.
5. Clientul deschide pagina de acord. Ea arată cine cere, unde te trimite înapoi răspunsul tău și comutatorul **Permite pornirea copiilor**, care pornește dezactivat. Alege **Permite** sau **Refuză**.

Fiecare client autentificat primește o placă lângă chei, cu sigla sa, jurnalul său, **Revocă** și **Permite pornirea copiilor**, și aceleași limite ca o cheie. Revocarea are efect imediat. Când același client se autentifică din nou, noua permisiune o înlocuiește pe cea veche, iar o permisiune pe care nimeni nu a folosit-o 30 de zile expiră. Pot fi autentificați până la 10 clienți în același timp, pe lângă cele 10 chei.

Pagina de acord acceptă o cerere doar de la un client înregistrat care indică exact una dintre adresele de întoarcere înregistrate: https, sau o adresă loopback pe orice port pentru un client de pe propriul tău calculator. Se acceptă doar fluxul cu cod de autorizare și PKCE (S256), iar răspunsul tău este legat de sesiunea ta, deci niciun alt site nu îl poate trimite în locul tău. Tokenurile de acces sunt valabile o oră. Un token de reîmprospătare este înlocuit la fiecare utilizare, iar dacă unul reapare după aceea, BombVault revocă permisiunea, pentru că altcineva are o copie. Un client care își repetă ultima reîmprospătare în 30 de secunde, pentru că răspunsul nu a ajuns niciodată la el, primește în schimb tokenuri noi. BombVault nu descarcă metadate ale clienților din internet, deci clienții se înregistrează prin înregistrarea dinamică a clienților.

### Alți clienți {#other-clients}

Merge orice client care vorbește Streamable HTTP:

- URL: adresa interfeței web plus `/mcp`, de exemplu `https://192.168.1.10:3443/mcp`.
- Cheia în `Authorization: Bearer <key>` sau în `X-API-Key: <key>`. Dacă vin amândouă, trebuie să conțină aceeași cheie.
- `POST` cu `Content-Type: application/json` și `Accept: application/json, text/event-stream`.
- Un singur mesaj JSON-RPC pe cerere; loturile (batch) sunt refuzate.
- Versiunile de protocol 2026-07-28, 2025-11-25, 2025-06-18 și 2025-03-26.

## TLS și certificate {#tls}

BombVault servește HTTPS cu un certificat emis de el însuși, iar la început acest certificat numește doar `localhost`, `127.0.0.1` și `::1`. Claude Code și `mcp-remote` îl refuză pe o adresă din rețeaua locală. Căile de ocolire, în ordinea care se potrivește celor mai multe instalări Unraid:

1. **Adaugă adresa în cardul MCP.** Când deschizi cardul prin HTTPS la o adresă pe care certificatul nu o numește, cardul spune asta și oferă **Adaugă această adresă la certificat**. BombVault emite atunci din nou certificatul, cu acea adresă inclusă (browserul te avertizează încă o dată, ca prima oară). Apoi dă clic pe **Descarcă certificatul**; fragmentele setează `NODE_EXTRA_CA_CERTS` pe fișierul descărcat, astfel încât clientul are încredere exact în acel certificat. Asta înseamnă și că orice client configurat cu un fișier descărcat anterior nu se mai conectează din clipa în care certificatul e emis din nou, pe acest computer și pe oricare altul, până primește fișierul nou.
2. **Un proxy invers cu un certificat de încredere** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Clientul vede atunci certificatul proxy-ului și nu mai are nevoie de nimic, iar cardul nu avertizează despre cel al BombVault.
3. **Tailscale.** `tailscale serve` în fața containerului sau integrarea Tailscale din Unraid îți dă un nume `ts.net` cu un certificat de încredere.
4. **`HTTP_ONLY=true`**, doar în spatele unui proxy care termină TLS sau într-o rețea în care ai încredere deplină. Trece toată interfața web pe HTTP simplu, cere o modificare în setările containerului și trimite cheia necriptată.

Nu seta niciodată `NODE_TLS_REJECT_UNAUTHORIZED=0`. Asta oprește verificarea certificatelor pentru tot ce vorbește cu acel proces Node.js.

Un proxy invers trebuie să transmită mai departe antetul `Authorization` (sau `X-API-Key`), lucru pe care proxy-urile îl fac dacă nu li se spune altfel, și nu are voie să pună `/mcp` în buffer sau să-l rescrie. Un bloc location pentru Nginx sau Nginx Proxy Manager care verifică și certificatul BombVault:

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

În spatele unui proxy, fiecare cerere poartă adresa proxy-ului. Cinci chei greșite de la un singur client configurat greșit blochează atunci timp de un minut toți clienții MCP din spatele acelui proxy. Trece proxy-ul în `TRUSTED_PROXY` (vezi [Configurare](configuration.md)) ca numărătoarea să se facă pe fiecare client.

## Modelul de securitate {#security}

- Fără o cheie activă și cu autentificarea prin OAuth oprită, `/mcp` răspunde `404`.
- Autentificarea prin OAuth este oferită doar cât timp este setată o parolă de conectare. Tokenurile, codurile și secretele clienților sunt stocate doar ca amprentă, iar un token funcționează doar pentru adresa pentru care a fost emis.
- Un client se poate înregistra de cel mult 10 ori pe oră de la o adresă, iar BombVault păstrează cel mult 100 de clienți înregistrați cu care nu s-a autentificat nimeni, fiecare timp de o zi. Codurile și tokenurile de reîmprospătare greșite se socotesc la aceeași blocare ca cheile greșite.
- Permisiunile se comportă ca cheile la restaurarea unei copii a configurației sau la schimbarea `APP_KEY`: după o restaurare, fiecare client trebuie să se autentifice din nou.
- Nicio adresă nu e scutită. Cererile de la `localhost`, de la gazda Unraid, de la un proxy invers sau de la `tailscale serve` au nevoie de o cheie ca oricare altele, chiar și când interfața web nu are parolă de autentificare.
- Cheile sunt stocate doar ca amprente, arătate o singură dată și pot fi redenumite, înlocuite și revocate. Până la 10 chei active, fiecare cu propriul comutator **Permite pornirea copiilor**.
- Fiecare creare, înlocuire, schimbare de permisiune și revocare trimite o notificare pe canalele tale de notificare, cu adresa de la care a venit, în afară de cazul în care notificările sunt oprite.
- 5 chei greșite pe minut de la aceeași adresă, apoi `429`. 120 de cereri pe minut și 12 copii pornite pe oră pentru fiecare cheie, plus așteptarea și protecția păstrării de mai sus.
- Cererile de la o pagină de browser cu altă origine sunt refuzate.
- Cât timp nu e setată o parolă de autentificare, nu se pot crea chei de la un nume de gazdă care pare public.
- Fiecare copie pornită de un asistent, precum și rulările de prune și off-site care rezultă din ea, sunt marcate "prin MCP" cu numele cheii în jurnalul de activitate, în panoul de erori și în notificarea copiei.
- Fiecare apel de unealtă e scris în jurnalul containerului cu id-ul cheii și ultimele ei patru caractere (niciodată cu numele) și e numărat în `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Restaurarea unei copii a configurației revocă toate cheile, pentru că baza de date restaurată poate conține chei pe care le-ai revocat după ce a fost salvată. Creează chei noi după aceea.
- O cheie nu mai funcționează când se schimbă `APP_KEY` (o reinstalare sau o restaurare în alt container). Cardul detectează asta și marchează cheia, iar **Înlocuiește cheia** îi dă din nou un secret valid.
- Tratează o cheie ca pe o parolă. Un client care nu poate citi cheia dintr-o variabilă de mediu, dintr-o solicitare sau dintr-un fișier al cheii o ține ca text simplu în configurația sau setările lui, iar dialogul lui spune asta. Pe un computer în care ai mai puțină încredere, alege mai bine o cheie doar pentru citire.

## Ce iese din mașină {#privacy}

Tot ce citește un asistent ajunge la furnizorul de IA din spatele lui: numele elementelor, programările, istoricul rulărilor cu mesajele de eroare, id-urile și orele punctelor de restaurare, numele motoarelor de baze de date și dimensiunile dumpurilor, activitatea în curs, cifrele de stocare, acoperirea și starea. BombVault scoate căile gazdei, locațiile depozitelor, numele gazdelor, datele de autentificare, comenzile de hook și cheile înainte ca ceva să iasă.

## Depanare {#troubleshooting}

| Ce vezi | Ce înseamnă |
|---|---|
| `404` | Nicio cheie activă și autentificarea prin OAuth este oprită, sau o cale greșită precum `/api/mcp`. Endpoint-ul este `/mcp`. |
| `401` | Cheia lipsește, e scrisă greșit, revocată sau înlocuită. Poate un proxy aruncă antetul `Authorization` (încearcă `X-API-Key`). Dacă cardul marchează cheia ca nemaifiind validă, `APP_KEY` s-a schimbat: înlocuiește cheia. |
| `403` | Cererea a venit de la o pagină de browser cu altă origine. Folosește un client desktop sau de linie de comandă. |
| `405` la GET | Normal. Punctul de conectare acceptă doar `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Clientul e prea vechi pentru Streamable HTTP. Actualizează-l. |
| `400` "batch requests are not accepted" | Clientul trimite loturi JSON-RPC. Trimite un mesaj pe cerere. |
| `429` | Prea multe chei greșite de la această adresă, sau mai mult de 120 de cereri pe minut cu o cheie. Așteaptă un minut și verifică dacă asistentul nu s-a blocat într-o buclă. |
| Erori cu "certificate", "self-signed" sau "unable to verify" | Clientul nu are încredere în certificatul BombVault. Vezi [TLS și certificate](#tls). |
| `busy` | Altă copie sau o sarcină de întreținere ocupă acel domeniu. Încearcă din nou după ce se termină. |
| `cooldown` | Acest element, acest domeniu sau Backup Everything a fost pornit prin MCP acum mai puțin de 15 minute. |
| `retention_guard` | Încă o copie MCP ar lăsa doar puncte de restaurare din MCP într-o fereastră "păstrează ultimele N", sau elementul a primit deja 4 copii prin MCP în ultimele 24 de ore, inclusiv cele eșuate și anulate. În primul caz următoarea copie programată face loc, în al doilea elementul e liber din nou la 24 de ore după cea mai veche dintre aceste copii. Din interfața web o poți porni oricând. |
| `rate_limited` | Cheia și-a consumat cele 12 porniri din această oră. |
| `not_permitted` la o pornire | Cheia poate doar să citească. Pornește **Permite pornirea copiilor** în card; nu e nevoie de reconectare. La o anulare înseamnă că rularea nu a fost pornită de această cheie. |
| `domain_off` | Acel tip de copie e oprit în setări. |
| `not_found` | BombVault nu protejează acel element. Adaugă-l întâi în interfața web; MCP nu creează niciodată configurație. |
| Clientul nu găsește serverul de autorizare | Autentificarea prin OAuth este oprită, nu este setată o parolă de conectare, sau proxy-ul nu lasă `/.well-known/` să treacă până la BombVault. |
| Pagina de acord spune că adresa de întoarcere nu este înregistrată | Clientul a trimis o adresă de întoarcere pe care nu a înregistrat-o. Elimină conectorul din client și adaugă-l din nou. |
| Un client autentificat primește `401` | Permisiunea lui a fost revocată, a expirat după 30 de zile fără utilizare, sau adresa publică s-a schimbat. Clientul se autentifică din nou. |

Nu seta variabila de mediu `MCPGODEBUG` pe container. Schimbă comportamentul bibliotecii MCP, iar o valoare greșită oprește BombVault la pornire înainte să scrie măcar o linie în jurnal.
