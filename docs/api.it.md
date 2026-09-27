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
| 403 | `not_permitted` | Il token può solo leggere, oppure non ha avviato quell'esecuzione |
| 404 | `not_found` | Elemento, esecuzione o anomalia inesistente |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | C'è già qualcosa in corso, il dominio è spento o non c'è niente da fare |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Un limite trattiene la richiesta; `Retry-After` dice quando riprovare |

Gli avvii seguono gli stessi limiti degli [avvii tramite MCP](mcp.md#starting-backups): 12 all'ora per token, 15 minuti tra due avvii dello stesso elemento, al massimo 4 avvii di un elemento in 24 ore e la protezione della conservazione. Gli ultimi tre contano insieme gli avvii tramite MCP e tramite API. Un token può fare 120 richieste al minuto. Cinque tentativi falliti da un indirizzo lo bloccano per un minuto.

## OpenAPI {#openapi}

BombVault fornisce una descrizione di queste route in `/api/v1/openapi.json` (OpenAPI 3.1). Non serve un token. Caricala in Swagger UI, Postman o un generatore di codice.
