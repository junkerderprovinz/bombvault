# API și integrări

BombVault are un mic API HTTP pentru scripturi, panouri și automatizarea casei. Citește aceleași lucruri pe care le arată panoul și poate porni o copie. Tot restul, cum ar fi restaurările, ștergerea copiilor și setările, rămâne în interfața web.

## Tokenuri {#tokens}

Fiecare cerere are nevoie de un token API, chiar și fără parolă de conectare. Creează unul în **Setări, Sistem, Tokenuri API**:

1. Scrie un nume care spune unde se folosește tokenul, de exemplu „Home Assistant” sau „Uptime Kuma”.
2. Pornește **Permite pornirea copiilor** dacă tokenul trebuie să poată porni copii. Fără asta poate doar să citească.
3. Apasă **Creează tokenul**. Tokenul apare o singură dată. BombVault păstrează doar o amprentă a lui, așa că copiază-l acum.

Trimite tokenul într-un antet, fie `Authorization: Bearer <token>`, fie `X-API-Key: <token>`. Un token începe cu `bvapi_`. Deschide doar API-ul: o cheie MCP nu funcționează aici, iar un token nu funcționează pentru MCP.

Fiecare token are o casetă cu numele, dacă poate porni copii, ultimele patru caractere, când și de unde a fost folosit ultima dată și apelurile de azi. Din casetă îl poți redenumi, îi poți schimba drepturile, îl poți înlocui sau revoca. **Jurnal** arată copiile pe care le-a pornit și ultimele apeluri. Restaurarea configurației BombVault dintr-o copie revocă toate tokenurile, pentru că acea copie poate conține tokenuri revocate între timp.

Fără parolă de conectare, oricine poate deschide interfața web poate crea și un token. Dacă deschizi BombVault sub un nume care pare public și nu e setată nicio parolă, de la acea adresă nu se pot crea tokenuri, la fel ca la [cheile MCP](mcp.md#switch-on).

## Puncte de acces {#endpoints}

| Rută | Ce returnează sau face | Token |
|---|---|---|
| `GET /api/v1/health` | Versiunea, numele instanței, dacă rulează o copie și ce poate face acest token | citire |
| `GET /api/v1/status` | Starea protecției pe domenii: ultima copie reușită, intervalul așteptat, verificările, următoarele rulări planificate | citire |
| `GET /api/v1/activity` | Ce rulează acum, cu fază și procent | citire |
| `GET /api/v1/items` | Fiecare element protejat cu programul, ce oprește o copie și ultima copie; `?domain=` pentru un domeniu | citire |
| `GET /api/v1/runs` | Istoricul rulărilor, cele mai noi primele; filtre `limit`, `domain`, `item`, `status`, `kind`, `since` | citire |
| `GET /api/v1/anomalies` | Anomalii cu un rezumat al celor deschise; filtre `state`, `severity`, `domain`, `limit` | citire |
| `GET /api/v1/anomalies/{id}` | O anomalie | citire |
| `GET /api/v1/storage/{domain}` | Istoricul mărimii, creșterea pe săptămână și spațiul liber al fiecărui depozit dintr-un domeniu | citire |
| `POST /api/v1/backups` | Copiază un element (`{"domain":"containers","item":"plex"}`) sau un domeniu întreg (`{"domain":"vms"}`) | pornire |
| `POST /api/v1/backups/everything` | Rulează Backup Everything | pornire |
| `POST /api/v1/runs/{id}/cancel` | Anulează o copie în curs pornită de acest token | pornire |

Domeniile sunt `containers`, `vms`, `files`, `zfs`, `flash` și `config`. Timpii sunt secunde Unix. Răspunsurile sunt aceleași ca la [instrumentele MCP](mcp.md#tools) cu același nume, așa că cele două rămân la fel.

O copie pornită aici este aceeași pe care o pornește interfața web: un container care rulează e oprit până se termină copia lui. Cererea se întoarce imediat, iar `/api/v1/activity` și `/api/v1/runs` arată cum merge.

## Exemple {#examples}

```sh
# Cum stau copiile?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Copiază un container acum.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Cu certificatul autosemnat propriu al BombVault, adaugă `--cacert bombvault-cert.pem` (fișierul primit prin **Descarcă certificatul** pe cardul MCP) sau `-k` într-o rețea de încredere.

## Erori și limite {#errors}

O eroare se întoarce ca `{"error": {"code": "...", "message": "..."}}`, cu starea potrivită:

| Stare | Coduri | Înțeles |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Un argument lipsește sau e greșit |
| 401 | `no_token`, `invalid_token` | Niciun token, sau nu unul activ |
| 403 | `not_permitted` | Tokenul poate doar citi, sau nu el a pornit rularea |
| 404 | `not_found` | Nu există acel element, acea rulare sau anomalie |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Rulează deja altceva, domeniul e oprit sau nu e nimic de făcut |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | O limită reține cererea; `Retry-After` spune când poți încerca din nou |

Pornirile respectă aceleași limite ca [pornirile prin MCP](mcp.md#starting-backups): 12 pe oră per token, 15 minute între două porniri ale aceluiași element, cel mult 4 porniri ale unui element în 24 de ore și protecția de retenție. Ultimele trei numără împreună pornirile prin MCP, prin API și din Home Assistant. Un token poate face 120 de cereri pe minut. Cinci încercări eșuate de la o adresă o blochează un minut.

## OpenAPI {#openapi}

BombVault servește o descriere a acestor rute la `/api/v1/openapi.json` (OpenAPI 3.1). Nu cere token. Încarc-o în Swagger UI, Postman sau un generator de cod.

## Home Assistant {#home-assistant}

BombVault poate apărea în Home Assistant ca dispozitiv, prin descoperirea MQTT. Home Assistant are nevoie de integrarea sa MQTT și de un broker, de exemplu add-on-ul Mosquitto. Nu e nevoie de nicio componentă proprie.

1. În BombVault, deschide **Setări, Sistem, Home Assistant**.
2. Introdu adresa și portul brokerului, iar utilizatorul și parola dacă le cere. Pornește **Folosește TLS** dacă brokerul folosește TLS, de obicei pe portul 8883; certificatul lui trebuie să fie valid pentru adresa introdusă.
3. Pornește **Conectează la Home Assistant** și apasă **Salvare**. Cardul arată când conexiunea e activă.

Dispozitivul se numește BombVault, sau BombVault cu numele instanței între paranteze, și are aceste entități:

| Entitate | Ce arată |
|---|---|
| Status | `ok`, `warning`, `failed` sau `off`, cel mai rău dintre domeniile pornite |
| Running job | Ce rulează acum, sau `idle` |
| Open anomalies | Câte anomalii sunt deschise |
| Next scheduled backup | Când începe următoarea copie planificată |
| *Domeniu* last backup | Când a rulat ultima copie reușită a domeniului |
| *Domeniu* last result | Cum s-a încheiat ultima sa copie |
| *Domeniu* repository free space | Spațiul liber acolo unde se află depozitul său principal, dacă BombVault îl poate citi |
| Back up *domeniu* | Un buton care copiază tot domeniul |

Numele entităților sunt în engleză, pentru că Home Assistant le preia așa cum le trimite BombVault. Fiecare domeniu pornit primește entități proprii, iar unul pe care îl oprești le pierde. Butoanele respectă aceleași limite ca [pornirile prin API](#errors). Oricine poate publica pe broker le poate apăsa, așa că pune o parolă brokerului sau oprește **Butoanele pornesc copii**.

BombVault își citește starea la fiecare 15 secunde și o publică atunci când s-a schimbat ceva, ca JSON sub `<prefix>/<nod>/state`. Prefixul este `bombvault` cât timp nu îl schimbi, iar nodul este un identificator scurt pe care BombVault îl alege o dată. Mesajele de descoperire merg la prefixul implicit al Home Assistant, `homeassistant`. Ambele sunt păstrate (retained). Un ultim mesaj (last will) marchează dispozitivul ca indisponibil dacă BombVault se oprește fără să anunțe. Dacă oprești legătura, BombVault scoate dispozitivul și entitățile lui din Home Assistant.
