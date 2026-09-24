# Server MCP

BombVault ha un server integrato per il Model Context Protocol (MCP), il protocollo con cui assistenti IA come Claude Code e Claude Desktop raggiungono strumenti esterni. Attraverso di esso un assistente può leggere come stanno i tuoi backup e, se lo permetti, avviare un backup o annullarne uno che ha avviato lui stesso. Resta spento finché non crei una chiave: senza una chiave attiva il punto di connessione `/mcp` risponde `404` a tutto.

## Cosa può fare un assistente e cosa no {#tools}

| Strumento | Cosa fa | Tipo |
|---|---|---|
| `get_health` | Versione, nome dell'istanza, se è in corso un backup e cosa può fare questa chiave | lettura |
| `get_status` | Stato di protezione per dominio: ultimo backup riuscito, intervallo previsto, verifiche e controlli off-site, prossime esecuzioni pianificate | lettura |
| `get_coverage` | Cosa protegge BombVault e cosa no, con il motivo per ciascuno | lettura |
| `list_items` | Ogni container, VM e set di cartelle protetto, la chiavetta flash e la configurazione dell'app, con pianificazione, cosa ferma un backup, l'ultimo backup e quanto è durato; i container di database riportano anche l'ultimo dump; compaiono anche i dataset ZFS, con l'esito del loro ultimo controllo | lettura |
| `list_runs` | Cronologia delle esecuzioni, le più recenti prima, filtrabile per dominio, elemento, stato, tipo e data | lettura |
| `list_restore_points` | Punti di ripristino di un elemento nel suo repository principale e, per un container, i suoi dump di database; un dataset ZFS ha un punto di ripristino per backup, con uno snapshot di ogni dataset sottostante | lettura |
| `get_activity` | Cosa è in esecuzione adesso, con fase e percentuale | lettura |
| `get_storage_stats` | Storico delle dimensioni del repository principale di un dominio e la sua crescita settimanale | lettura |
| `list_anomalies` | Anomalie notate da BombVault nei backup, filtrabili per stato, gravità e dominio, con un riepilogo di ciò che è aperto | lettura |
| `get_anomaly` | Una di queste segnalazioni, con la nota lasciata quando è stata confermata | lettura |
| `start_backup` | Esegue subito il backup di un elemento | avvio |
| `start_domain_backup` | Esegue il backup di ogni elemento protetto di un dominio | avvio |
| `start_backup_everything` | Avvia il passaggio Backup Everything | avvio |
| `cancel_backup` | Annulla un backup in corso avviato da questa chiave | annullamento |

Restano nell'interfaccia web: i ripristini di ogni tipo (compresi scaricare, salvare o importare un dump di database), l'eliminazione di backup, prune, unlock, verifiche ed esercitazioni, la replica off-site, le impostazioni, le credenziali e le chiavi MCP, e l'annullamento di un backup avviato dalla pianificazione, dall'interfaccia web o da un'altra chiave. Lo stesso vale per confermare un'anomalia o segnarla come prevista, cosa che si fa nella pagina **Anomalie**. Il motivo: le risposte degli strumenti contengono nomi e messaggi di errore del tuo server, e ognuno di essi potrebbe contenere un testo scritto per manipolare l'assistente. Un assistente che ci casca può al massimo avviare un backup entro i limiti qui sotto, o annullarne uno che ha avviato lui stesso.

Se il repository principale di un elemento è remoto (S3, REST, SFTP, rclone), `list_restore_points` lo contatta e la chiamata può richiedere un po' di tempo. Le copie off-site non si possono elencare tramite MCP. Cosa guardano i controlli delle anomalie è descritto in [Funzionalità](features.md), e come un elemento ZFS tiene uno snapshot per ogni dataset in [Dataset ZFS](zfs-datasets.md#contents).

## Cosa fa un backup avviato {#starting-backups}

Il backup di un assistente è lo stesso che avvia l'interfaccia web. Un container in esecuzione viene fermato finché il suo backup non termina, insieme ai container impostati per fermarsi con lui. Una VM con il metodo "graceful" viene spenta e riavviata. Un dataset ZFS ferma i container impostati per lui mentre viene preso il suo snapshot. I set di cartelle, la chiavetta flash e la configurazione continuano a funzionare. Dopo, BombVault applica la politica di conservazione e può copiare nel repository off-site. `list_items` dice all'assistente cosa ferma un elemento e quanto è durato il suo ultimo backup, e le descrizioni degli strumenti gli chiedono di dirtelo prima di avviare qualsiasi cosa.

Poiché un backup ferma dei servizi e fa uscire vecchi punti di ripristino, gli avvii tramite MCP sono limitati:

- 12 backup avviati all'ora per chiave.
- 15 minuti tra due avvii MCP dello stesso elemento, dello stesso dominio o di Backup Everything.
- Al massimo 4 avvii MCP dello stesso elemento in 24 ore.
- **Protezione della conservazione.** Quando un dominio conserva un numero fisso di punti di ripristino (solo "conserva gli ultimi N", senza regola giornaliera, settimanale o mensile, in locale o su una destinazione off-site), ogni nuovo backup fa uscire il più vecchio. BombVault rifiuta allora un avvio MCP di un elemento i cui N-1 backup riusciti più recenti sono stati tutti avviati tramite MCP. Nell'insieme conservato resta quindi sempre almeno un punto di ripristino creato dalla pianificazione o da te. Con "conserva l'ultimo" (N = 1) un assistente non può fare il backup di quell'elemento. Il prossimo backup pianificato fa di nuovo spazio.

Un avvio di dominio o di Backup Everything lascia fuori gli elementi trattenuti da un limite e li nomina nella risposta. L'interfaccia web e la pianificazione non sono toccate da nessuno di questi limiti. Il budget orario vive in memoria, quindi un riavvio di BombVault lo azzera.

## Attivarlo {#switch-on}

1. Apri **Impostazioni, Sistema, Server MCP** e fai clic su **Nuova chiave**.
2. Dai alla chiave un nome che dica dove viene usata, per esempio "Claude Code sul portatile". Con una chiave per client puoi revocarne una senza toccare le altre.
3. Lascia attivo **Consenti di avviare backup**, oppure disattivalo per una chiave che deve solo leggere. Puoi cambiarlo più tardi sulla riga della chiave, e la modifica vale dalla richiesta successiva dell'assistente, senza riconnessione.
4. Fai clic su **Crea chiave**. La chiave viene mostrata una sola volta. BombVault ne conserva solo un'impronta e non può mostrarla di nuovo, quindi copiala subito oppure prendi uno degli snippet sotto, che a quel punto contengono la chiave vera.

Senza password di accesso l'interfaccia web stessa è aperta a tutta la tua rete, e chi può aprirla può anche creare una chiave. La scheda lo segnala. Se apri BombVault con un nome che sembra pubblico (per esempio `bombvault.example.com` dietro un reverse proxy) e non è impostata una password di accesso, da quell'indirizzo non si possono creare né sostituire chiavi, così nessuna pagina web su Internet può indurre il tuo browser a crearne una. Imposta una password di accesso, oppure apri BombVault dal suo indirizzo IP o da un nome locale come `tower` o `tower.local`.

## Collegare un client {#clients}

La scheda mostra snippet pronti per l'indirizzo con cui l'hai aperta: scegli il client e copia lo snippet. Qui sotto trovi cosa fanno gli snippet e le forme che la scheda non mostra.

### Claude Code {#claude-code}

Esegui una volta in un terminale il comando della scheda. Con un certificato di cui il tuo computer si fida, ha questo aspetto:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Controlla la connessione con `/mcp` dentro Claude Code. `--scope user` salva la chiave nella tua configurazione utente invece che in un file di progetto.

Il comando contiene la chiave, e la tua shell potrebbe conservarlo nella cronologia. Per evitarlo, metti un `.mcp.json` nella cartella del progetto e tieni la chiave in una variabile d'ambiente. Claude Code sostituisce `${BOMBVAULT_MCP_KEY}` quando legge il file:

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

Imposta `BOMBVAULT_MCP_KEY` dove parte Claude Code, per esempio nel profilo della shell, modificato con un editor di testo invece che digitato al prompt. Non fare mai commit di un `.mcp.json` con la chiave scritta dentro.

Con il certificato proprio di BombVault (vedi [TLS e certificati](#tls)) il comando della scheda avvia invece `mcp-remote` e indica a Node.js il certificato scaricato:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Gli apici singoli impediscono alla shell di espandere la variabile; ci pensa `mcp-remote`. La stessa forma funziona in `.mcp.json`: usa la voce di Claude Desktop qui sotto e togli `BOMBVAULT_MCP_KEY` dal suo `env`, così la chiave arriva dal tuo ambiente.

### Claude Desktop {#claude-desktop}

Claude Desktop raggiunge BombVault tramite `mcp-remote`, che richiede Node.js su quel computer. Apri il file di configurazione in Claude Desktop da **Settings, Developer, Edit Config**. Si trova in `%APPDATA%\Claude\claude_desktop_config.json` su Windows e in `~/Library/Application Support/Claude/claude_desktop_config.json` su macOS. Aggiungi la voce della scheda dentro `"mcpServers"`, accanto ai server già presenti, e riavvia Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` serve solo per il certificato proprio di BombVault. Dietro un certificato di cui il tuo computer si fida già, toglilo.
- `--allow-http` viene aggiunto solo per un indirizzo `http://` in chiaro.
- L'intestazione è scritta `X-API-Key:${BOMBVAULT_MCP_KEY}`, senza spazio dopo i due punti e con la chiave in `env`. Su alcuni sistemi `mcp-remote` spezza un valore di `--header` al primo spazio, e una chiave scritta dopo uno spazio andrebbe persa.

### Connettori personalizzati nelle impostazioni di Claude {#custom-connectors}

I connettori aggiunti nelle impostazioni di Claude stesso (su claude.ai e nell'elenco dei connettori di Claude Desktop) non sono ancora supportati. Questi connettori vengono contattati dal cloud di Anthropic, quindi serve loro un indirizzo HTTPS pubblico, e accedono tramite OAuth. Non possono inviare una chiave fissa, e BombVault offre solo chiavi fisse, senza accesso OAuth. Mettere BombVault su Internet per loro non servirebbe. Usa Claude Code, oppure Claude Desktop tramite `mcp-remote` come sopra.

### Altri client {#other-clients}

Va bene qualsiasi client che parli Streamable HTTP:

- URL: l'indirizzo dell'interfaccia web più `/mcp`, per esempio `https://192.168.1.10:3443/mcp`.
- La chiave in `Authorization: Bearer <key>` oppure in `X-API-Key: <key>`. Se arrivano entrambe, devono contenere la stessa chiave.
- `POST` con `Content-Type: application/json` e `Accept: application/json, text/event-stream`.
- Un solo messaggio JSON-RPC per richiesta; i batch vengono rifiutati.
- Versioni del protocollo 2026-07-28, 2025-11-25, 2025-06-18 e 2025-03-26.

## TLS e certificati {#tls}

BombVault serve HTTPS con un certificato emesso da sé, e all'inizio quel certificato nomina solo `localhost`, `127.0.0.1` e `::1`. Claude Code e `mcp-remote` lo rifiutano su un indirizzo della rete locale. Le vie d'uscita, nell'ordine che va bene per la maggior parte delle installazioni Unraid:

1. **Aggiungere l'indirizzo nella scheda MCP.** Se apri la scheda in HTTPS su un indirizzo che il certificato non nomina, lo dice e offre **Aggiungi questo indirizzo al certificato**. BombVault emette di nuovo il suo certificato con quell'indirizzo (il browser avvisa ancora una volta, come la prima). Poi fai clic su **Scarica certificato**; gli snippet impostano `NODE_EXTRA_CA_CERTS` sul file scaricato, così il client si fida esattamente di quel certificato.
2. **Un reverse proxy con un certificato attendibile** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Il client vede allora il certificato del proxy e non ha bisogno d'altro, e la scheda non avvisa riguardo a quello di BombVault.
3. **Tailscale.** `tailscale serve` davanti al container, o l'integrazione Tailscale di Unraid, ti dà un nome `ts.net` con un certificato attendibile.
4. **`HTTP_ONLY=true`**, solo dietro un proxy che termina il TLS o in una rete di cui ti fidi del tutto. Porta tutta l'interfaccia web su HTTP in chiaro, richiede una modifica alle impostazioni del container e invia la chiave senza cifratura.

Non impostare mai `NODE_TLS_REJECT_UNAUTHORIZED=0`. Disattiva il controllo dei certificati per tutto ciò con cui parla quel processo Node.js.

Un reverse proxy deve lasciar passare l'intestazione `Authorization` (o `X-API-Key`), cosa che i proxy fanno se non gli si dice altrimenti, e non deve bufferizzare né riscrivere `/mcp`. Un blocco location per Nginx o Nginx Proxy Manager che controlla anche il certificato di BombVault:

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

Dietro un proxy ogni richiesta porta l'indirizzo del proxy. Cinque chiavi sbagliate da un solo client configurato male bloccano allora per un minuto tutti i client MCP dietro quel proxy. Indica il proxy in `TRUSTED_PROXY` (vedi [Configurazione](configuration.md)) per contare per client.

## Modello di sicurezza {#security}

- Senza una chiave attiva, `/mcp` risponde `404`.
- Nessun indirizzo è esente. Le richieste da `localhost`, dall'host Unraid, da un reverse proxy o da `tailscale serve` hanno bisogno di una chiave come tutte le altre, anche quando l'interfaccia web non ha una password di accesso.
- Le chiavi sono salvate solo come impronta, mostrate una sola volta, e si possono rinominare, sostituire e revocare. Fino a 10 chiavi attive, ognuna con il proprio interruttore **Può avviare backup**.
- Ogni creazione, sostituzione, modifica dei permessi e revoca invia una notifica tramite i tuoi canali di notifica, con l'indirizzo da cui è arrivata, a meno che le notifiche siano disattivate.
- 5 chiavi sbagliate al minuto per indirizzo, poi `429`. 120 richieste al minuto e 12 backup avviati all'ora per chiave, più l'attesa e la protezione della conservazione descritte sopra.
- Le richieste da una pagina del browser di un'altra origine vengono rifiutate.
- Finché non è impostata una password di accesso, non si possono creare chiavi da un nome host che sembra pubblico.
- Ogni backup avviato da un assistente, e le esecuzioni di prune e di copia off-site che ne derivano, è segnato "via MCP" con il nome della chiave nel registro attività, nel pannello degli errori e nella notifica del backup.
- Ogni chiamata a uno strumento viene scritta nel log del container con l'id della chiave e i suoi ultimi quattro caratteri (mai il nome) e contata in `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Ripristinare un backup della configurazione revoca tutte le chiavi, perché il database ripristinato può contenere chiavi che hai revocato dopo il suo salvataggio. Poi crea chiavi nuove.
- Una chiave smette di funzionare quando cambia `APP_KEY` (una reinstallazione o un ripristino su un altro container). La scheda lo rileva e segna la chiave, e **Sostituisci chiave** le ridà un segreto valido.
- Tratta una chiave come una password. Claude Code e Claude Desktop la conservano in chiaro nella loro configurazione. Su un computer di cui ti fidi meno, preferisci una chiave di sola lettura.

## Cosa esce dalla macchina {#privacy}

Tutto ciò che un assistente legge va al fornitore di IA che sta dietro: nomi degli elementi, pianificazioni, cronologia delle esecuzioni con i messaggi di errore, id e orari dei punti di ripristino, nomi dei motori di database e dimensioni dei dump, attività in corso, dati di archiviazione, copertura e stato. BombVault toglie i percorsi dell'host, le posizioni dei repository, i nomi host, le credenziali, i comandi hook e le chiavi prima che qualcosa esca.

## Risoluzione dei problemi {#troubleshooting}

| Cosa vedi | Cosa significa |
|---|---|
| `404` | Nessuna chiave attiva, oppure un percorso sbagliato come `/api/mcp`. Il punto di connessione è `/mcp`. |
| `401` | La chiave manca, è scritta male, revocata o sostituita. Forse un proxy scarta l'intestazione `Authorization` (prova `X-API-Key`). Se la scheda segna la chiave come non più valida, `APP_KEY` è cambiata: sostituisci la chiave. |
| `403` | La richiesta è arrivata da una pagina del browser di un'altra origine. Usa un client desktop o a riga di comando. |
| `405` su GET | Normale. Il punto di connessione accetta solo `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Il client è troppo vecchio per Streamable HTTP. Aggiornalo. |
| `400` "batch requests are not accepted" | Il client invia batch JSON-RPC. Invia un messaggio per richiesta. |
| `429` | Troppe chiavi sbagliate da questo indirizzo, oppure più di 120 richieste al minuto con una chiave. Aspetta un minuto e controlla che l'assistente non sia bloccato in un ciclo. |
| Errori con "certificate", "self-signed" o "unable to verify" | Il client non si fida del certificato di BombVault. Vedi [TLS e certificati](#tls). |
| `busy` | Un altro backup o un'attività di manutenzione occupa quel dominio. Riprova quando ha finito. |
| `cooldown` | Questo elemento, questo dominio o Backup Everything è stato avviato tramite MCP meno di 15 minuti fa. |
| `retention_guard` | Un altro backup MCP lascerebbe solo punti di ripristino MCP in una finestra "conserva gli ultimi N". Il prossimo backup pianificato fa spazio, oppure avvialo dall'interfaccia web. |
| `rate_limited` | La chiave ha usato i suoi 12 avvii di quest'ora. |
| `not_permitted` su un avvio | La chiave è di sola lettura. Attiva **Può avviare backup** nella scheda; non serve riconnettersi. Su un annullamento significa che l'esecuzione non è stata avviata da questa chiave. |
| `domain_off` | Quel tipo di backup è disattivato nelle impostazioni. |
| `not_found` | BombVault non protegge quell'elemento. Aggiungilo prima nell'interfaccia web; MCP non crea mai configurazione. |

Non impostare la variabile d'ambiente `MCPGODEBUG` sul container. Cambia il comportamento della libreria MCP, e un valore malformato ferma BombVault all'avvio prima ancora che scriva una sola riga di log.
