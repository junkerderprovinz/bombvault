# API e integrazioni

BombVault ha una piccola API HTTP per script, dashboard e domotica. Legge le stesse cose che mostra la panoramica e può avviare un backup. Tutto il resto, come i ripristini, l'eliminazione dei backup e le impostazioni, resta nell'interfaccia web.

## Token {#tokens}

Ogni richiesta ha bisogno di un token API, anche quando non è impostata alcuna password di accesso. Creane uno in **Impostazioni, Integrazioni, Token API**:

1. Scrivi un nome che dica dove viene usato il token, per esempio "Home Assistant" o "Uptime Kuma".
2. Attiva **Consenti di avviare backup** se il token deve poter avviare backup. Senza, il token può solo leggere.
3. Clicca **Crea token**. Il token viene mostrato una volta sola. BombVault ne conserva solo un'impronta, quindi copialo subito.

Invia il token in un header, come `Authorization: Bearer <token>` oppure come `X-API-Key: <token>`. Un token inizia con `bvapi_`. Apre solo l'API: una chiave MCP qui non funziona, e un token non funziona per MCP.

Ogni token ha un riquadro con il suo nome, se può avviare backup, gli ultimi quattro caratteri, quando è stato usato l'ultima volta e da dove, e le sue chiamate di oggi. Sul riquadro puoi rinominarlo, cambiare ciò che può fare, sostituirlo o revocarlo. **Registro** mostra i backup che ha avviato e le sue chiamate recenti. Ripristinare la configurazione di BombVault da un backup revoca ogni token, perché il backup potrebbe contenere token che hai revocato dopo.

Senza una password di accesso, chiunque riesca ad aprire l'interfaccia web può anche creare un token. Se apri BombVault con un nome che sembra pubblico e non è impostata alcuna password, da quell'indirizzo non si possono creare token, la stessa regola valida per le [chiavi MCP](mcp.md#switch-on).

## Endpoint {#endpoints}

| Route | Cosa restituisce o fa | Token |
|---|---|---|
| `GET /api/v1/health` | Versione, nome dell'istanza, se è in corso un backup e cosa può fare questo token | lettura |
| `GET /api/v1/status` | Stato di protezione per dominio: ultimo backup riuscito, intervallo previsto, controlli, prossime esecuzioni pianificate | lettura |
| `GET /api/v1/activity` | Cosa è in corso in questo momento, con fase e percentuale | lettura |
| `GET /api/v1/items` | Ogni elemento protetto con la sua pianificazione, ciò che un backup ferma e il suo ultimo backup. `?domain=` per un solo dominio | lettura |
| `GET /api/v1/runs` | Cronologia delle esecuzioni, la più recente per prima. Filtri `limit`, `domain`, `item`, `status`, `kind`, `since` | lettura |
| `GET /api/v1/anomalies` | Anomalie con un riepilogo di ciò che è aperto. Filtri `state`, `severity`, `domain`, `limit` | lettura |
| `GET /api/v1/anomalies/{id}` | Un'anomalia | lettura |
| `GET /api/v1/storage/{domain}` | Storico delle dimensioni, crescita a settimana e spazio libero di ogni repository di un dominio | lettura |
| `POST /api/v1/backups` | Salva un elemento (`{"domain":"containers","item":"plex"}`) o un intero dominio (`{"domain":"vms"}`) | avvio |
| `POST /api/v1/backups/everything` | Esegue il passaggio Esegui il backup di tutto ora | avvio |
| `POST /api/v1/runs/{id}/cancel` | Annulla un backup in corso avviato da questo token | avvio |

I domini sono `containers`, `vms`, `files`, `zfs`, `flash` e `config`. Gli orari sono secondi unix. Le risposte sono le stesse degli [strumenti MCP](mcp.md#tools) con gli stessi nomi, così restano allineate.

Un backup avviato è lo stesso backup che avvia l'interfaccia web: un container in esecuzione viene fermato finché il suo backup non è finito. La richiesta ritorna subito, e `/api/v1/activity` e `/api/v1/runs` mostrano come procede.

## Esempi {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Con il certificato autofirmato di BombVault aggiungi `--cacert bombvault-cert.pem` (il file che ti dà **Scarica certificato** nella scheda MCP) oppure `-k` su una rete di cui ti fidi.

## Errori e limiti {#errors}

Un errore torna come `{"error": {"code": "...", "message": "..."}}` con uno stato corrispondente:

| Stato | Codici | Significato |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Un argomento manca o è sbagliato |
| 401 | `no_token`, `invalid_token` | Nessun token, o non uno attivo |
| 403 | `not_permitted`, `forbidden_origin` | Il token può solo leggere, l'esecuzione non è stata avviata da lui, oppure la richiesta arriva da una pagina di un'altra origine |
| 404 | `not_found` | Nessun elemento, esecuzione o anomalia di questo tipo |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Sta già girando altro, il dominio è disattivato oppure non c'è nulla da fare |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Un limite trattiene la richiesta. `Retry-After` dice quando riprovare |

Gli avvii seguono gli stessi limiti degli [avvii tramite MCP](mcp.md#starting-backups): 12 all'ora per token, 15 minuti tra due avvii dello stesso elemento, al massimo 4 avvii di un elemento in 24 ore e la protezione della conservazione. Gli ultimi tre contano insieme gli avvii tramite MCP, API e Home Assistant. Un token può fare 120 richieste al minuto. Cinque tentativi falliti dallo stesso indirizzo lo bloccano per un minuto.

## OpenAPI {#openapi}

BombVault pubblica una descrizione di queste route in `/api/v1/openapi.json` (OpenAPI 3.1). Non richiede alcun token. Caricala in Swagger UI, in Postman o in un generatore di codice.

## Home Assistant {#home-assistant}

BombVault può comparire in Home Assistant come dispositivo, tramite MQTT discovery. Home Assistant ha bisogno della sua integrazione MQTT e di un broker, per esempio l'add-on Mosquitto. Non è coinvolto alcun componente personalizzato.

1. In BombVault apri **Impostazioni, Integrazioni** e attiva **Home Assistant** nella scheda **Servizi**. Sotto compaiono i campi per il broker.
2. Inserisci l'indirizzo e la porta del broker, e il nome utente e la password se li richiede. Attiva **Usa TLS** se il broker accetta TLS, di solito sulla porta 8883. Il suo certificato deve essere valido per l'indirizzo che hai inserito. Se cambi l'indirizzo, la porta o il nome utente, inserisci di nuovo la password: BombVault non passa quella salvata a un altro broker o utente.
3. Clicca **Salva**. La riga mostra quando la connessione è attiva.

Il dispositivo si chiama BombVault, oppure BombVault con il nome dell'istanza tra parentesi, e ha queste entità:

| Entità | Cosa mostra |
|---|---|
| Status | `ok`, `warning`, `overdue`, `failed` oppure `off`, il peggiore tra i domini attivati. Un dominio in attesa del primo backup conta solo quando quel backup era previsto, e solo un dominio aggiornato rende lo stato `ok`. `overdue` significa, come nella panoramica, che l'ultimo backup di un dominio è più vecchio di due intervalli |
| Running job | Cosa gira ora, oppure `idle` |
| Open anomalies | Quante anomalie sono aperte |
| Next scheduled backup | Quando parte il prossimo backup pianificato |
| *Domain* last backup | Quando è stato eseguito l'ultimo backup riuscito del dominio |
| *Domain* last result | Come si è concluso il suo ultimo backup |
| *Domain* repository free space | Lo spazio libero dove si trova il suo repository primario, se BombVault riesce a leggerlo |
| Back up *domain* | Un pulsante che salva l'intero dominio |

Ogni dominio attivato ottiene entità proprie, e un dominio che disattivi le perde. I pulsanti compaiono quando attivi **I pulsanti avviano backup**. In una nuova installazione è disattivato. Seguono gli stessi limiti degli [avvii tramite l'API](#errors). In più BombVault gestisce una pressione per dominio alla volta e non più di sei al minuto, e ignora una pressione che il broker ha conservato come messaggio retained. Chiunque possa pubblicare sul broker può premerli, quindi proteggi il broker con una password.

BombVault legge il proprio stato ogni 15 secondi e lo pubblica quando qualcosa è cambiato, come JSON sotto `<prefix>/<node>/state`. Il prefisso è `bombvault` finché non lo cambi, e il nodo è un breve id che BombVault sceglie una volta. I messaggi di discovery vanno al prefisso predefinito di Home Assistant, `homeassistant`. Entrambi sono retained. Un last will segna il dispositivo come non disponibile se BombVault si ferma senza avvisare. Disattivare il collegamento rimuove da Home Assistant il dispositivo e le sue entità.

### Esempio: una scheda e un avviso {#home-assistant-example}

Home Assistant ricava gli id delle entità dai nomi, quindi il dispositivo BombVault ottiene id come `sensor.bombvault_status`. Con un nome di istanza tra parentesi anche gli id lo contengono, perciò controllali prima in **Developer Tools, States**. Una scheda per la dashboard:

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

Un'automazione che avvisa il telefono quando lo stato diventa `failed` oppure `overdue`:

```yaml
alias: BombVault needs you
trigger:
  - platform: state
    entity_id: sensor.bombvault_status
    to:
      - failed
      - overdue
action:
  - service: notify.mobile_app_my_phone
    data:
      message: "BombVault status: {{ trigger.to_state.state }}. Open the overview."
```

## Prometheus {#metrics}

**Esponi /metrics** nella scheda **Servizi** in **Impostazioni, Integrazioni** pubblica stato dei backup, dimensioni e timestamp per Prometheus, Grafana o Uptime Kuma, in `/metrics` sulla porta dell'interfaccia web. È disattivato di default. **Token di scrape (facoltativo)** fa inviare allo scraper un bearer token. Le label non contengono segreti né percorsi.

## Widget per dashboard {#dashboard-widget}

La scheda **Widget per dashboard** in **Impostazioni, Integrazioni** genera un token di sola lettura e ti dà un indirizzo per qualsiasi dashboard che mostri un iframe, come Homepage, Organizr o Heimdall. L'indirizzo apre una piccola pagina in `/widget` con solo il registro attività in tempo reale. Il token concede quel registro e nient'altro, e **Disattiva** lo revoca subito. La pagina incorporata è solo in inglese.

## Trovare BombVault in rete {#mdns}

BombVault annuncia la sua interfaccia web sulla rete locale tramite mDNS, il protocollo dietro Bonjour e Avahi. Un browser lo raggiunge allora come `https://bombvault.local:3443`, oppure `http://bombvault.local:3000` con `HTTP_ONLY`, e i browser di servizi lo elencano come servizio web con il sottotipo `_bombvault`. I suoi record TXT portano la versione e il percorso. L'interruttore è **Trova in rete** nella scheda **Servizi** in **Impostazioni, Integrazioni** ed è attivo di default. Se un altro dispositivo usa già il nome, BombVault prende `bombvault-2.local` e così via, e la scheda mostra l'indirizzo ottenuto. Quando BombVault si ferma o disattivi l'annuncio, lo comunica alla rete, così i browser tolgono subito la voce.

Se l'annuncio raggiunge la tua rete dipende da come è collegato il container:

- **bridge**, il valore predefinito nel template Unraid: l'annuncio resta dentro la rete di Docker e nessuno nella LAN lo vede. Apri BombVault con l'indirizzo dell'host come prima.
- **br0** o un'altra rete macvlan o ipvlan: il container ha un proprio indirizzo nella LAN e l'annuncio lo raggiunge.
- **host**: l'annuncio esce dalle interfacce dell'host, accanto a quello di Unraid. Salta i bridge di Docker e libvirt.

Vengono annunciati solo indirizzi IPv4.
