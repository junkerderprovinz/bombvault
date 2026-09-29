# API e integrazioni

BombVault ha una piccola API HTTP per script, dashboard e domotica. Legge le stesse cose che mostra la dashboard e può avviare un backup. Tutto il resto, come ripristini, eliminazione di backup e impostazioni, resta nell'interfaccia web.

## Token {#tokens}

Ogni richiesta ha bisogno di un token API, anche senza password di accesso. Crealo in **Impostazioni, Sistema, Token API**:

1. Scrivi un nome che dica dove si usa il token, per esempio "Home Assistant" o "Uptime Kuma".
2. Attiva **Consenti di avviare backup** se il token deve poter avviare backup. Altrimenti può solo leggere.
3. Fai clic su **Crea token**. Il token compare una volta sola. BombVault ne tiene solo un'impronta, quindi copialo adesso.

Invia il token in un'intestazione, `Authorization: Bearer <token>` oppure `X-API-Key: <token>`. Un token inizia con `bvapi_`. Apre solo l'API: una chiave MCP qui non funziona, e un token non funziona per MCP.

Ogni token ha un riquadro con il nome, se può avviare backup, gli ultimi quattro caratteri, quando e da dove è stato usato l'ultima volta e le chiamate di oggi. Dal riquadro puoi rinominarlo, cambiare cosa può fare, sostituirlo o revocarlo. **Registro** mostra i backup che ha avviato e le ultime chiamate. Ripristinare la configurazione di BombVault da un backup revoca tutti i token, perché il backup può contenere token revocati in seguito.

Senza password di accesso, chiunque possa aprire l'interfaccia web può anche creare un token. Se apri BombVault con un nome dall'aspetto pubblico e non c'è password, da quell'indirizzo non si possono creare token, come per le [chiavi MCP](mcp.md#switch-on).

## Endpoint {#endpoints}

| Route | Cosa restituisce o fa | Token |
|---|---|---|
| `GET /api/v1/health` | Versione, nome dell'istanza, se un backup è in corso e cosa può fare questo token | lettura |
| `GET /api/v1/status` | Stato di protezione per dominio: ultimo backup riuscito, intervallo previsto, controlli, prossime esecuzioni | lettura |
| `GET /api/v1/activity` | Cosa è in corso adesso, con fase e percentuale | lettura |
| `GET /api/v1/items` | Ogni elemento protetto con pianificazione, cosa ferma un backup e l'ultimo backup; `?domain=` per un dominio | lettura |
| `GET /api/v1/runs` | Cronologia delle esecuzioni, le più recenti prima; filtri `limit`, `domain`, `item`, `status`, `kind`, `since` | lettura |
| `GET /api/v1/anomalies` | Anomalie con un riepilogo di quelle aperte; filtri `state`, `severity`, `domain`, `limit` | lettura |
| `GET /api/v1/anomalies/{id}` | Un'anomalia | lettura |
| `GET /api/v1/storage/{domain}` | Andamento della dimensione, crescita settimanale e spazio libero di ogni repository di un dominio | lettura |
| `POST /api/v1/backups` | Esegue il backup di un elemento (`{"domain":"containers","item":"plex"}`) o di un intero dominio (`{"domain":"vms"}`) | avvio |
| `POST /api/v1/backups/everything` | Avvia Backup Everything | avvio |
| `POST /api/v1/runs/{id}/cancel` | Annulla un backup in corso avviato da questo token | avvio |

I domini sono `containers`, `vms`, `files`, `zfs`, `flash` e `config`. Gli orari sono secondi Unix. Le risposte sono quelle degli [strumenti MCP](mcp.md#tools) con lo stesso nome, così le due restano allineate.

Un backup avviato qui è lo stesso che avvia l'interfaccia web: un container in esecuzione viene fermato fino alla fine del suo backup. La richiesta torna subito, e `/api/v1/activity` e `/api/v1/runs` mostrano come procede.

## Esempi {#examples}

```sh
# Come vanno i backup?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Backup di un container adesso.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Con il certificato autofirmato di BombVault aggiungi `--cacert bombvault-cert.pem` (il file che ottieni con **Scarica certificato** sulla scheda MCP) oppure `-k` su una rete di cui ti fidi.

## Errori e limiti {#errors}

Un errore torna come `{"error": {"code": "...", "message": "..."}}` con lo stato corrispondente:

| Stato | Codici | Significato |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Un argomento manca o è sbagliato |
| 401 | `no_token`, `invalid_token` | Nessun token, o non uno attivo |
| 403 | `not_permitted`, `forbidden_origin` | Il token può solo leggere, oppure non ha avviato quell'esecuzione, oppure la richiesta arriva da una pagina di un'altra origine |
| 404 | `not_found` | Elemento, esecuzione o anomalia inesistente |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | C'è già qualcosa in corso, il dominio è spento o non c'è niente da fare |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Un limite trattiene la richiesta; `Retry-After` dice quando riprovare |

Gli avvii seguono gli stessi limiti degli [avvii tramite MCP](mcp.md#starting-backups): 12 all'ora per token, 15 minuti tra due avvii dello stesso elemento, al massimo 4 avvii di un elemento in 24 ore e la protezione della conservazione. Gli ultimi tre contano insieme gli avvii tramite MCP, tramite API e da Home Assistant. Un token può fare 120 richieste al minuto. Cinque tentativi falliti da un indirizzo lo bloccano per un minuto.

## OpenAPI {#openapi}

BombVault fornisce una descrizione di queste route in `/api/v1/openapi.json` (OpenAPI 3.1). Non serve un token. Caricala in Swagger UI, Postman o un generatore di codice.

## Home Assistant {#home-assistant}

BombVault può comparire in Home Assistant come dispositivo, grazie al rilevamento MQTT. Home Assistant ha bisogno della sua integrazione MQTT e di un broker, per esempio il componente aggiuntivo Mosquitto. Non serve alcun componente personalizzato.

1. In BombVault apri **Impostazioni, Sistema, Home Assistant**.
2. Inserisci indirizzo e porta del broker, e nome utente e password se li chiede. Attiva **Usa TLS** se il broker usa TLS, di solito sulla porta 8883; il suo certificato deve essere valido per l'indirizzo inserito. Se cambi indirizzo, porta o nome utente, inserisci di nuovo la password: BombVault non passa quella salvata a un altro broker o utente.
3. Attiva **Collega a Home Assistant** e fai clic su **Salva**. La scheda mostra quando la connessione è attiva.

Il dispositivo si chiama BombVault, oppure BombVault con il nome dell'istanza tra parentesi, e ha queste entità:

| Entità | Cosa mostra |
|---|---|
| Status | `ok`, `warning`, `failed` o `off`, il peggiore dei domini attivi |
| Running job | Cosa è in corso adesso, oppure `idle` |
| Open anomalies | Quante anomalie sono aperte |
| Next scheduled backup | Quando parte il prossimo backup pianificato |
| *Dominio* last backup | Quando è stato eseguito l'ultimo backup riuscito del dominio |
| *Dominio* last result | Come si è concluso il suo ultimo backup |
| *Dominio* repository free space | Lo spazio libero dove si trova il suo repository principale, se BombVault riesce a leggerlo |
| Back up *dominio* | Un pulsante che esegue il backup dell'intero dominio |

I nomi delle entità sono in inglese, perché Home Assistant li riprende così come BombVault li invia. Ogni dominio attivo ha le sue entità, e un dominio spento le perde. I pulsanti compaiono quando attivi **I pulsanti avviano backup**, che su una nuova installazione è spento. Seguono gli stessi limiti degli [avvii tramite l'API](#errors). In più BombVault accetta una sola pressione alla volta per dominio e al massimo sei al minuto, e ignora una pressione che il broker ha conservato come messaggio retained. Chiunque possa pubblicare sul broker può premerli, quindi proteggi il broker con una password.

BombVault legge il suo stato ogni 15 secondi e lo pubblica quando qualcosa è cambiato, come JSON sotto `<prefisso>/<nodo>/state`. Il prefisso è `bombvault` finché non lo cambi, e il nodo è un breve identificativo che BombVault sceglie una volta. I messaggi di rilevamento vanno al prefisso predefinito di Home Assistant, `homeassistant`. Entrambi sono conservati (retained). Un ultimo messaggio (last will) segna il dispositivo come non disponibile se BombVault si ferma senza avvisare. Disattivare il collegamento rimuove il dispositivo e le sue entità da Home Assistant.

## Trovare BombVault in rete {#mdns}

BombVault annuncia la sua interfaccia web sulla rete locale tramite mDNS, il protocollo dietro Bonjour e Avahi. Un browser lo raggiunge così come `https://bombvault.local:3443`, oppure `http://bombvault.local:3000` con `HTTP_ONLY`, e i browser di servizi lo elencano come servizio web con il sottotipo `_bombvault`. I suoi record TXT riportano la versione e il percorso. L'interruttore si trova in **Impostazioni, Sistema, Trova in rete** ed è attivo di serie. Se un altro dispositivo usa già il nome, BombVault prende `bombvault-2.local` e così via, e la scheda mostra l'indirizzo ottenuto. Quando BombVault si ferma o disattivi l'annuncio, lo comunica alla rete e i browser tolgono subito la voce.

Se l'annuncio raggiunge la tua rete dipende da come è collegato il container:

- **bridge**, l'impostazione predefinita del modello Unraid: l'annuncio resta nella rete di Docker e nessuno sulla LAN lo vede. Apri BombVault con l'indirizzo dell'host come prima.
- **br0** o un'altra rete macvlan o ipvlan: il container ha un suo indirizzo sulla LAN e l'annuncio la raggiunge.
- **host**: l'annuncio esce dalle interfacce dell'host, accanto a quello di Unraid. I bridge di Docker e di libvirt restano esclusi.

Vengono annunciati solo indirizzi IPv4.
