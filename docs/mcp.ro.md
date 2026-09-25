# Server MCP

BombVault are un server integrat pentru Model Context Protocol (MCP), protocolul prin care asistenți IA precum Claude Code și Claude Desktop ajung la unelte externe. Prin el, un asistent poate citi cum stau copiile tale de rezervă și, dacă îi permiți, poate porni o copie sau poate anula una pe care a pornit-o chiar el. Serverul e oprit până când creezi o cheie: fără o cheie activă, punctul de conectare `/mcp` răspunde `404` la orice.

## Ce poate și ce nu poate face un asistent {#tools}

| Unealtă | Ce face | Tip |
|---|---|---|
| `get_health` | Versiunea, numele instanței, dacă rulează o copie și ce are voie să facă această cheie | citire |
| `get_status` | Starea protecției pe fiecare domeniu: ultima copie reușită, intervalul așteptat, verificările și controalele off-site, următoarele rulări programate | citire |
| `get_coverage` | Ce protejează BombVault și ce nu, cu motivul pentru fiecare | citire |
| `list_items` | Fiecare container, VM și set de foldere protejat, stick-ul flash și configurația aplicației, cu programarea, ce oprește o copie, ultima copie și cât a durat; containerele de baze de date arată și ultimul dump; apar și seturile de date ZFS, cu rezultatul ultimei lor verificări | citire |
| `list_runs` | Istoricul rulărilor, cele mai noi primele, filtrabil după domeniu, element, stare, tip și timp | citire |
| `list_restore_points` | Punctele de restaurare ale unui element din depozitul lui principal și, pentru un container, dumpurile lui de baze de date; un set de date ZFS are câte un punct de restaurare pentru fiecare copie, cu un snapshot al fiecărui set de date de sub el | citire |
| `get_activity` | Ce rulează chiar acum, cu fază și procent | citire |
| `get_storage_stats` | Istoricul de dimensiune al depozitului principal al unui domeniu și creșterea lui pe săptămână | citire |
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
- **Protecția păstrării.** Când un domeniu păstrează un număr fix de puncte de restaurare (doar "păstrează ultimele N", fără regulă zilnică, săptămânală sau lunară, local sau pe o destinație off-site), fiecare copie nouă scoate afară cea mai veche. BombVault refuză atunci o pornire MCP a unui element ale cărui cele mai noi N-1 copii reușite au fost pornite toate prin MCP. Astfel, în setul păstrat rămâne mereu cel puțin un punct de restaurare făcut de programare sau de tine. Cu "păstrează ultima 1", un asistent nu poate face deloc copia acelui element. Următoarea copie programată face din nou loc.

O pornire de domeniu sau de Backup Everything lasă pe dinafară elementele reținute de o limită și le numește în răspuns. Interfața web și programarea nu sunt atinse de niciuna dintre aceste limite. Bugetul orar stă în memorie, așa că o repornire a BombVault îl readuce la zero.

## Pornire {#switch-on}

1. Deschide **Setări, Sistem, Server MCP** și dă clic pe **Cheie nouă**.
2. Dă cheii un nume care spune unde e folosită, de exemplu "Claude Code pe laptop". Cu o cheie pentru fiecare client poți revoca una fără să le atingi pe celelalte.
3. Lasă pornit **Permite pornirea copiilor** sau oprește-l pentru o cheie care trebuie doar să citească. Poți schimba asta mai târziu pe dala cheii, iar schimbarea se aplică de la următoarea cerere a asistentului, fără reconectare.
4. Dă clic pe **Creează cheia**. Cheia apare o singură dată. BombVault păstrează doar o amprentă a ei și nu o mai poate arăta, așa că copiaz-o acum sau ia unul dintre fragmentele de dedesubt, care conțin atunci cheia reală.

Fără parolă de autentificare, interfața web însăși e deschisă pentru toată lumea din rețeaua ta, iar cine o poate deschide poate crea și o cheie. Cardul spune asta. Dacă deschizi BombVault sub un nume care pare public (de exemplu `bombvault.example.com` în spatele unui proxy invers) și nu e setată nicio parolă de autentificare, de la acea adresă nu se pot crea și nici înlocui chei, ca nicio pagină web de pe internet să nu-ți poată face browserul să creeze una. Setează o parolă de autentificare sau deschide BombVault prin adresa IP ori printr-un nume local precum `tower` sau `tower.local`.

## Cheile tale și jurnalul lor {#keys}

Fiecare cheie are propria dală pe card. Arată numele cheii, dacă poate porni copii sau doar citește, ultimele patru caractere ale cheii, când a fost creată sau înlocuită ultima dată, când a folosit-o ultima dată un client și câte apeluri a făcut azi. Pe dală redenumești cheia, îi schimbi permisiunea, o înlocuiești sau o revoci. O cheie revocată trece în lista cheilor revocate, unde o poți șterge definitiv când nicio rulare din istoric nu o mai numește.

**Jurnal** pe o dală deschide ce a făcut acea cheie. Întâi vin copiile pe care le-a pornit, fiecare cu starea ei și un link către acea rulare în jurnalul de activitate de pe tabloul de bord. Dedesubt sunt apelurile ei, cele mai noi primele, cu instrumentul și rezultatul apelului. Un refuz spune de ce: cheia poate doar citi, protecția de păstrare a oprit copia, rula deja altă copie, elementul a fost copiat prin MCP acum câteva minute sau cheia a trimis prea multe cereri. O anulare trimite la rularea la care se referea.

BombVault păstrează cele mai noi 200 de intrări ale fiecărei chei cel mult 30 de zile. Pentru fiecare apel salvează instrumentul, rezultatul și rularea numită de o anulare. Nu salvează niciodată ce a trimis asistentul, nici cheia sau amprenta ei. Pachetul de diagnostic doar numără intrările, iar un export al setărilor le lasă deoparte.

## Conectarea unui client {#clients}

Cardul arată fragmente gata făcute pentru adresa la care l-ai deschis: alege clientul și copiază fragmentul. Restul secțiunii explică ce fac fragmentele și dă formele pe care cardul nu le arată.

### Claude Code {#claude-code}

Rulează o dată comanda de pe card într-un terminal. Cu un certificat în care calculatorul tău are încredere, arată așa:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Verifică legătura cu `/mcp` în Claude Code. `--scope user` pune cheia în configurația ta de utilizator, nu într-un fișier de proiect.

Comanda conține cheia, iar shell-ul tău o poate păstra în istoric. Ca să eviți asta, pune un `.mcp.json` în folderul proiectului și ține cheia într-o variabilă de mediu. Claude Code înlocuiește `${BOMBVAULT_MCP_KEY}` când citește fișierul:

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

Setează `BOMBVAULT_MCP_KEY` acolo unde pornește Claude Code, de exemplu în profilul shell-ului, editat într-un editor de text, nu tastat la prompt. Nu face niciodată commit unui `.mcp.json` în care cheia e scrisă direct.

Cu certificatul propriu al BombVault (vezi [TLS și certificate](#tls)), comanda de pe card rulează în schimb `mcp-remote` și îi arată lui Node.js certificatul descărcat:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Ghilimelele simple împiedică shell-ul să extindă variabila; asta o face `mcp-remote` singur. Aceeași formă merge și în `.mcp.json`: folosește intrarea pentru Claude Desktop de mai jos și scoate `BOMBVAULT_MCP_KEY` din `env`-ul ei, iar cheia vine atunci din mediul tău.

### Claude Desktop {#claude-desktop}

Claude Desktop ajunge la BombVault prin `mcp-remote`, care are nevoie de Node.js pe acel calculator. Deschide fișierul de configurare în Claude Desktop din **Settings, Developer, Edit Config**. Se află în `%APPDATA%\Claude\claude_desktop_config.json` pe Windows și în `~/Library/Application Support/Claude/claude_desktop_config.json` pe macOS. Adaugă intrarea de pe card în `"mcpServers"`, lângă serverele care sunt deja acolo, și repornește Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` e acolo doar pentru certificatul propriu al BombVault. În spatele unui certificat în care calculatorul tău are deja încredere, scoate-l.
- `--allow-http` se adaugă doar pentru o adresă `http://` simplă.
- Antetul se scrie `X-API-Key:${BOMBVAULT_MCP_KEY}`, fără spațiu după două puncte și cu cheia în `env`. Pe unele sisteme, `mcp-remote` taie o valoare `--header` la primul spațiu, iar o cheie scrisă după un spațiu s-ar pierde.

### Conectori proprii în setările Claude {#custom-connectors}

Conectorii adăugați în setările lui Claude însuși (pe claude.ai și în lista de conectori din Claude Desktop) nu sunt încă suportați. Acești conectori sunt contactați din cloudul Anthropic, deci au nevoie de o adresă HTTPS publică, și se autentifică prin OAuth. Nu pot trimite o cheie fixă, iar BombVault oferă doar chei fixe, fără autentificare OAuth. Să pui BombVault pe internet pentru ei nu ar ajuta. Folosește Claude Code sau Claude Desktop prin `mcp-remote`, ca mai sus.

### Alți clienți {#other-clients}

Merge orice client care vorbește Streamable HTTP:

- URL: adresa interfeței web plus `/mcp`, de exemplu `https://192.168.1.10:3443/mcp`.
- Cheia în `Authorization: Bearer <key>` sau în `X-API-Key: <key>`. Dacă vin amândouă, trebuie să conțină aceeași cheie.
- `POST` cu `Content-Type: application/json` și `Accept: application/json, text/event-stream`.
- Un singur mesaj JSON-RPC pe cerere; loturile (batch) sunt refuzate.
- Versiunile de protocol 2026-07-28, 2025-11-25, 2025-06-18 și 2025-03-26.

## TLS și certificate {#tls}

BombVault servește HTTPS cu un certificat emis de el însuși, iar la început acest certificat numește doar `localhost`, `127.0.0.1` și `::1`. Claude Code și `mcp-remote` îl refuză pe o adresă din rețeaua locală. Căile de ocolire, în ordinea care se potrivește celor mai multe instalări Unraid:

1. **Adaugă adresa în cardul MCP.** Când deschizi cardul prin HTTPS la o adresă pe care certificatul nu o numește, cardul spune asta și oferă **Adaugă această adresă la certificat**. BombVault emite atunci din nou certificatul, cu acea adresă inclusă (browserul te avertizează încă o dată, ca prima oară). Apoi dă clic pe **Descarcă certificatul**; fragmentele setează `NODE_EXTRA_CA_CERTS` pe fișierul descărcat, astfel încât clientul are încredere exact în acel certificat.
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

- Fără o cheie activă, `/mcp` răspunde `404`.
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
- Tratează o cheie ca pe o parolă. Claude Code și Claude Desktop o păstrează în text simplu în configurația lor. Pe un calculator în care ai mai puțină încredere, folosește mai bine o cheie doar pentru citire.

## Ce iese din mașină {#privacy}

Tot ce citește un asistent ajunge la furnizorul de IA din spatele lui: numele elementelor, programările, istoricul rulărilor cu mesajele de eroare, id-urile și orele punctelor de restaurare, numele motoarelor de baze de date și dimensiunile dumpurilor, activitatea în curs, cifrele de stocare, acoperirea și starea. BombVault scoate căile gazdei, locațiile depozitelor, numele gazdelor, datele de autentificare, comenzile de hook și cheile înainte ca ceva să iasă.

## Depanare {#troubleshooting}

| Ce vezi | Ce înseamnă |
|---|---|
| `404` | Nicio cheie activă, sau o cale greșită precum `/api/mcp`. Punctul de conectare este `/mcp`. |
| `401` | Cheia lipsește, e scrisă greșit, revocată sau înlocuită. Poate un proxy aruncă antetul `Authorization` (încearcă `X-API-Key`). Dacă cardul marchează cheia ca nemaifiind validă, `APP_KEY` s-a schimbat: înlocuiește cheia. |
| `403` | Cererea a venit de la o pagină de browser cu altă origine. Folosește un client desktop sau de linie de comandă. |
| `405` la GET | Normal. Punctul de conectare acceptă doar `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Clientul e prea vechi pentru Streamable HTTP. Actualizează-l. |
| `400` "batch requests are not accepted" | Clientul trimite loturi JSON-RPC. Trimite un mesaj pe cerere. |
| `429` | Prea multe chei greșite de la această adresă, sau mai mult de 120 de cereri pe minut cu o cheie. Așteaptă un minut și verifică dacă asistentul nu s-a blocat într-o buclă. |
| Erori cu "certificate", "self-signed" sau "unable to verify" | Clientul nu are încredere în certificatul BombVault. Vezi [TLS și certificate](#tls). |
| `busy` | Altă copie sau o sarcină de întreținere ocupă acel domeniu. Încearcă din nou după ce se termină. |
| `cooldown` | Acest element, acest domeniu sau Backup Everything a fost pornit prin MCP acum mai puțin de 15 minute. |
| `retention_guard` | Încă o copie MCP ar lăsa doar puncte de restaurare din MCP într-o fereastră "păstrează ultimele N". Următoarea copie programată face loc, sau pornește copia din interfața web. |
| `rate_limited` | Cheia și-a consumat cele 12 porniri din această oră. |
| `not_permitted` la o pornire | Cheia poate doar să citească. Pornește **Permite pornirea copiilor** în card; nu e nevoie de reconectare. La o anulare înseamnă că rularea nu a fost pornită de această cheie. |
| `domain_off` | Acel tip de copie e oprit în setări. |
| `not_found` | BombVault nu protejează acel element. Adaugă-l întâi în interfața web; MCP nu creează niciodată configurație. |

Nu seta variabila de mediu `MCPGODEBUG` pe container. Schimbă comportamentul bibliotecii MCP, iar o valoare greșită oprește BombVault la pornire înainte să scrie măcar o linie în jurnal.
