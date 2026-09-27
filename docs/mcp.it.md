# Server MCP

BombVault ha un server integrato per il Model Context Protocol (MCP), il protocollo con cui assistenti IA come Claude Code e Claude Desktop raggiungono strumenti esterni. Tramite questo server un assistente può leggere lo stato dei tuoi backup e, se lo consenti, avviare un backup o annullarne uno che ha avviato lui. È spento finché non crei una chiave o non attivi l'[accesso tramite OAuth](#oauth): fino ad allora l'endpoint `/mcp` risponde `404` a tutto.

## Cosa può fare un assistente e cosa no {#tools}

| Strumento | Cosa fa | Tipo |
|---|---|---|
| `get_health` | Versione, nome dell'istanza, se è in corso un backup e cosa può fare questa chiave | lettura |
| `get_status` | Stato di protezione per dominio: ultimo backup riuscito, intervallo previsto, verifiche e controlli off-site, prossime esecuzioni pianificate, backup in attesa che l'app sia inattiva | lettura |
| `get_coverage` | Cosa protegge BombVault e cosa no, con il motivo per ciascuno | lettura |
| `list_items` | Ogni container, VM e set di cartelle protetto, la chiavetta flash e la configurazione dell'app, con pianificazione, cosa ferma un backup, l'ultimo backup e quanto è durato; i container di database riportano anche l'ultimo dump; compaiono anche i dataset ZFS, con l'esito del loro ultimo controllo; un container ricreato con altre impostazioni dall'ultimo backup elenca cosa è cambiato | lettura |
| `list_runs` | Cronologia delle esecuzioni, le più recenti prima, filtrabile per dominio, elemento, stato, tipo e data; un backup lento frenato da una sola cosa la nomina | lettura |
| `list_restore_points` | Punti di ripristino di un elemento nel suo repository principale e, per un container, i suoi dump di database; un dataset ZFS ha un punto di ripristino per backup, con uno snapshot di ogni dataset sottostante | lettura |
| `get_activity` | Cosa è in esecuzione adesso, con fase e percentuale | lettura |
| `get_storage_stats` | Storico delle dimensioni del repository principale di un dominio e la sua crescita settimanale, con lo spazio usato, libero e totale sul disco o sul remoto di ciascuno dei suoi repository | lettura |
| `get_size_breakdown` | Quali cartelle e file occupano spazio nel backup più recente di un container, una VM o un insieme di cartelle, e quanto ne ha aggiunto l'ultimo backup | lettura |
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

1. Apri **Impostazioni, Sistema, Server MCP** e fai clic sul pulsante del tuo client. Un client che non è nell'elenco si collega tramite **Altro client**.
2. In **Chiave** lascia **Nuova chiave** e il nome proposto, cioè quello del client, oppure scrivine uno che dica dove si usa la chiave, per esempio «Claude Code sul portatile». Una chiave per client ti permette di revocarne una senza toccare le altre. **Chiave esistente** dà al client una chiave che hai creato prima.
3. Attiva **Consenti di avviare backup** per una chiave che deve poter avviare backup; senza, può solo leggere. Puoi cambiarlo più tardi sul riquadro della chiave, e la modifica vale dalla richiesta successiva dell'assistente, senza riconnessione.
4. Fai clic su **Crea chiave**. La chiave viene mostrata una volta sola. BombVault ne conserva solo un'impronta e non può mostrarla di nuovo, quindi copiala adesso. Se chiudi la finestra prima che il client abbia usato la chiave, la scheda continua a mostrarla finché non confermi di averla copiata.

Senza password di accesso l'interfaccia web stessa è aperta a tutta la tua rete, e chi può aprirla può anche creare una chiave. La scheda lo segnala. Se apri BombVault con un nome che sembra pubblico (per esempio `bombvault.example.com` dietro un reverse proxy) e non è impostata una password di accesso, da quell'indirizzo non si possono creare né sostituire chiavi, così nessuna pagina web su Internet può indurre il tuo browser a crearne una. Imposta una password di accesso, oppure apri BombVault dal suo indirizzo IP o da un nome locale come `tower` o `tower.local`.

## Le tue chiavi e il loro registro {#keys}

Ogni chiave ha una scheda sua nella card. Mostra il nome della chiave, se può avviare backup o solo leggere, gli ultimi quattro caratteri della chiave, quando è stata creata o sostituita l'ultima volta, quando un client l'ha usata l'ultima volta e quante chiamate ha fatto oggi. Dalla scheda rinomini la chiave, ne cambi il permesso, la sostituisci o la revochi. Una chiave revocata passa nell'elenco delle chiavi revocate, dove puoi eliminarla per sempre quando nessuna esecuzione nella cronologia la nomina più.

Accanto al nome, il riquadro mostra il logo del client per cui la chiave è stata creata. Una chiave creata tramite **Altro client**, o prima che la scheda elencasse i client, mostra invece una chiave.

**Registro** su una scheda apre ciò che ha fatto quella chiave. Prima vengono i backup che ha avviato, ognuno con il suo stato e un link a quell'esecuzione nel registro attività della dashboard. Sotto ci sono le sue chiamate, le più recenti prima, con lo strumento e l'esito. Un rifiuto dice il motivo: la chiave può solo leggere, la protezione della conservazione ha trattenuto il backup, era già in corso un altro backup, l'elemento è stato salvato tramite MCP pochi minuti fa, oppure la chiave ha inviato troppe richieste. Un annullamento rimanda all'esecuzione a cui si riferiva.

BombVault conserva le voci di ogni chiave per 30 giorni al massimo: gli ultimi 500 avvii e annullamenti riusciti e, accanto a questi, le ultime 200 altre chiamate (letture, rifiuti ed errori). Così un assistente che interroga di continuo un backup in corso, o che ripete una chiamata rifiutata, non può spingere il suo avvio fuori dal registro. Per ogni chiamata salva lo strumento, l'esito e l'esecuzione nominata da un annullamento. Non salva mai ciò che l'assistente ha inviato, né la chiave o la sua impronta. Il pacchetto di diagnostica conta soltanto le voci, e un'esportazione delle impostazioni le lascia fuori.

## Collegare un client {#clients}

Ogni client ha un pulsante sulla scheda, sotto **Su questo computer** o **Nel cloud**. Il pulsante apre una finestra in tre passaggi: la chiave; la configurazione per quel client, con l'indirizzo con cui hai aperto la scheda, un pulsante per copiarla, dove si trova la configurazione e, con il certificato proprio di BombVault, ciò che serve al client per fidarsene; e l'attesa della prima chiamata del client. La finestra osserva l'ultimo uso della chiave e diventa verde quando arriva quella chiamata.

La finestra tiene la chiave lontana da ogni riga di comando. Se il client sa leggerla da una variabile d'ambiente (`BOMBVAULT_MCP_KEY`), da una richiesta mascherata o da un file suo, la configurazione si limita a nominarla. Se il client non ha un modo simile, la chiave sta nel suo file di configurazione o nelle sue impostazioni, e la finestra lo dice. Se la documentazione di un client non dice come tratta un certificato che non conosce, la finestra scrive quel passaggio come ciò che va fatto se il client rifiuta il certificato di BombVault.

| Client | Configurazione | Da dove arriva la chiave |
|---|---|---|
| AnythingLLM | file di configurazione | il file di configurazione |
| Antigravity | file di configurazione | variabile d'ambiente |
| Claude Code | comando | file della chiave |
| Claude Desktop | file di configurazione | file della chiave |
| Cline | file di configurazione | il file di configurazione |
| Codex CLI | file di configurazione | variabile d'ambiente |
| Continue | file di configurazione | `~/.continue/.env` |
| Copilot CLI | file di configurazione | il file di configurazione |
| Cursor | file di configurazione | variabile d'ambiente |
| Gemini CLI | file di configurazione | variabile d'ambiente |
| GitHub Copilot (VS Code) | file di configurazione | richiesta mascherata |
| Goose | file di configurazione | variabile d'ambiente |
| Jan | modulo nell'app | le impostazioni dell'app |
| JetBrains (AI Assistant, Junie) | file di configurazione | il file di configurazione |
| Kimi Code | file di configurazione | il file di configurazione |
| LM Studio | file di configurazione | il file di configurazione |
| Mistral Vibe | file di configurazione | variabile d'ambiente |
| Msty | modulo nell'app | le impostazioni dell'app |
| n8n | modulo nell'app | le credenziali di n8n |
| Open WebUI | modulo nell'app | le impostazioni dell'app |
| opencode | file di configurazione | variabile d'ambiente |
| Perplexity (Mac) | modulo nell'app | file della chiave |
| Qwen Code | file di configurazione | variabile d'ambiente |
| Roo Code | file di configurazione | variabile d'ambiente |
| Visual Studio | file di configurazione | il file di configurazione |
| Warp | file di configurazione | il file di configurazione |
| Windsurf | file di configurazione | variabile d'ambiente |
| Zed | file di configurazione | il file di configurazione |
| Grok | modulo, nel cloud | i server del fornitore |
| Le Chat | modulo, nel cloud | i server del fornitore |
| ChatGPT | accesso tramite OAuth, nel cloud | un token di accesso, vedi [sotto](#oauth) |
| Claude (claude.ai) | accesso tramite OAuth, nel cloud | un token di accesso, vedi [sotto](#oauth) |

Le sezioni seguenti spiegano più nel dettaglio la configurazione di Claude Code e Claude Desktop ed elencano ciò che serve a qualsiasi altro client.

### Claude Code {#claude-code}

Claude Code raggiunge BombVault tramite `mcp-remote`, che richiede Node.js su quel computer. Prima salva la chiave in un file di testo a parte, su una sola riga:

```text
X-API-Key: <your key>
```

Poi esegui una volta in un terminale il comando della scheda, con il percorso di quel file. Dietro un certificato di cui il tuo computer si fida, ha questo aspetto:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Con il certificato proprio di BombVault (vedi [TLS e certificati](#tls)) il comando indica anche a Node.js il certificato scaricato:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Controlla la connessione con `/mcp` dentro Claude Code. `--scope user` rende BombVault disponibile in tutti i tuoi progetti. Claude Code conserva solo il percorso del file della chiave, così la chiave non compare né nel comando e nella cronologia della shell, né nell'elenco dei processi. Tieni il file dove solo tu puoi leggerlo, e fuori da qualsiasi cartella di cui fai commit. `@latest` fa sì che `npx` scarichi un `mcp-remote` aggiornato; altrimenti verrebbe usato uno più vecchio installato globalmente, che non conosce `--header-file`.

Non scrivere `${BOMBVAULT_MCP_KEY}` negli argomenti di `mcp-remote` per Claude Code. Claude Code sostituisce un riferimento del genere con il proprio ambiente prima di avviare `mcp-remote`, quindi la chiave finisce sulla riga di comando di quel processo, dove altri programmi e utenti del computer possono leggerla.

Senza Node.js, e solo dietro un certificato di cui il tuo computer si fida, Claude Code può collegarsi da solo. Metti un `.mcp.json` nella cartella del progetto:

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

Imposta `BOMBVAULT_MCP_KEY` dove parte Claude Code, per esempio sotto `"env"` in `~/.claude/settings.json` o nel profilo della shell, modificato con un editor di testo invece che digitato al prompt. Qui il riferimento è sicuro, perché Claude Code non avvia un secondo processo che se lo porterebbe dietro. Con il certificato proprio di BombVault questa strada non funziona: la connessione che Claude Code apre da sé lo rifiuta anche con `NODE_EXTRA_CA_CERTS` impostato. Non fare mai commit di un `.mcp.json` con la chiave scritta dentro.

### Claude Desktop {#claude-desktop}

Claude Desktop raggiunge BombVault tramite `mcp-remote`, che richiede Node.js su quel computer. Prima salva la chiave in un file di testo a sé, su una sola riga, come descritto per [Claude Code](#claude-code). Apri il file di configurazione in Claude Desktop da **Settings, Developer, Edit Config**. Si trova in `%APPDATA%\Claude\claude_desktop_config.json` su Windows e in `~/Library/Application Support/Claude/claude_desktop_config.json` su macOS. Aggiungi la voce della scheda dentro `"mcpServers"`, accanto ai server già presenti, e riavvia Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` serve solo per il certificato proprio di BombVault. Dietro un certificato di cui il tuo computer si fida già, toglilo.
- `--allow-http` viene aggiunto solo per un indirizzo `http://` in chiaro.
- Su Windows scrivi i percorsi con le barre normali, per esempio `C:/Users/sam/bombvault-key.txt`, perché una barra rovesciata singola non è JSON valido. Tieni il percorso del file della chiave senza spazi: Claude Desktop su Windows passa a `npx` un percorso con uno spazio in due pezzi.
- La configurazione nomina solo il file della chiave, così la chiave non compare né lì né nell'elenco dei processi. Tieni il file dove solo tu puoi leggerlo.

### Client nel cloud {#cloud-clients}

ChatGPT, Claude su claude.ai, Grok e Le Chat chiamano BombVault dai server del loro fornitore, quindi BombVault deve essere raggiungibile da Internet con un certificato riconosciuto pubblicamente, per esempio dietro un reverse proxy; Le Chat rifiuta quelli autofirmati. Un accesso sul proxy può proteggere l'interfaccia web, ma `/mcp` deve arrivare a BombVault senza: questi servizi non sanno accedere a un proxy, e BombVault controlla da sé la loro chiave o il loro token. Grok e Le Chat inviano una chiave fissa, e i loro pulsanti li configurano come gli altri. ChatGPT, e Claude su claude.ai nella maggior parte delle organizzazioni, si collegano solo con un accesso tramite OAuth, descritto qui sotto.

### Accesso tramite OAuth {#oauth}

Per un client che non accetta una chiave, BombVault è il suo server di autorizzazione OAuth. Il client si registra da solo, ti manda su una pagina di BombVault, e lì accedi con la tua password di accesso (e il secondo fattore, se l'hai configurato) e gli dai il consenso. Il client riceve poi un token che vale solo per l'endpoint MCP di questo BombVault, e lo rinnova da solo.

1. Imposta una password di accesso in **Impostazioni, Sistema**. Senza, BombVault non offre alcun accesso, perché non ci sarebbe nessuno a cui chiedere il consenso.
2. Rendi BombVault raggiungibile da Internet in https con un certificato di cui i browser si fidano, di solito tramite un reverse proxy. Il client chiama `/mcp`, `/oauth/` e `/.well-known/` dai propri server, quindi un proxy con un proprio accesso deve lasciar passare questi tre percorsi fino a BombVault. La pagina di consenso in `/oauth/authorize` si apre nel tuo browser e può restare dietro l'accesso del proxy. Indica anche il proxy in `TRUSTED_PROXY` (vedi [Configurazione](configuration.md)). BombVault limita le registrazioni dei client per indirizzo e, senza questo, ogni client sembra arrivare dal proxy.
3. Nella scheda MCP attiva **Accesso tramite OAuth** e inserisci l'**Indirizzo pubblico**: l'indirizzo https senza percorso, per esempio `https://backup.example.com`. Ogni token è legato a questo indirizzo, quindi dopo una modifica ogni client deve accedere di nuovo.
4. Fai clic sul pulsante di ChatGPT o di Claude. La finestra mostra l'**URL del connettore**, cioè l'indirizzo pubblico seguito da `/mcp`, e dove va inserito in quel client. In ChatGPT attiva la modalità sviluppatore in **Impostazioni, App e connettori, Impostazioni avanzate**, scegli **Crea**, incolla l'URL del connettore come URL del server MCP e scegli OAuth come autenticazione. Su claude.ai apri **Impostazioni, Connettori, Aggiungi connettore personalizzato**, incolla l'URL del connettore, lascia vuoti ID client e secret OAuth e scegli **Connetti**.
5. Il client apre la pagina di consenso. Mostra chi chiede, dove ti riporta la tua risposta e l'interruttore **Consenti di avviare backup**, che parte spento. Scegli **Consenti** o **Nega**.

Ogni client che ha effettuato l'accesso ottiene un riquadro accanto alle chiavi, con il suo logo, il suo registro, **Revoca** e **Consenti di avviare backup**, e gli stessi limiti di una chiave. La revoca ha effetto subito. Quando lo stesso client accede di nuovo, il nuovo consenso sostituisce il precedente, e un consenso che nessuno ha usato per 30 giorni scade. Possono essere collegati fino a 10 client alla volta, oltre alle 10 chiavi.

La pagina di consenso accetta una richiesta solo da un client registrato che indica esattamente uno dei suoi indirizzi di ritorno registrati: https, oppure un indirizzo di loopback su qualsiasi porta per un client sul tuo computer. Viene accettato solo il flusso con codice di autorizzazione e PKCE (S256), e la tua risposta è legata alla tua sessione, quindi nessun altro sito può inviarla al posto tuo. I token di accesso durano un'ora. Un refresh token viene sostituito a ogni utilizzo, e se ricompare dopo, BombVault revoca il consenso, perché qualcun altro ne ha una copia. Un client che ripete il suo ultimo rinnovo entro 30 secondi, perché la risposta non gli è mai arrivata, riceve invece token nuovi. BombVault non scarica metadati dei client da Internet, quindi i client si registrano tramite la registrazione dinamica dei client.

### Altri client {#other-clients}

Va bene qualsiasi client che parli Streamable HTTP:

- URL: l'indirizzo dell'interfaccia web più `/mcp`, per esempio `https://192.168.1.10:3443/mcp`.
- La chiave in `Authorization: Bearer <key>` oppure in `X-API-Key: <key>`. Se arrivano entrambe, devono contenere la stessa chiave.
- `POST` con `Content-Type: application/json` e `Accept: application/json, text/event-stream`.
- Un solo messaggio JSON-RPC per richiesta; i batch vengono rifiutati.
- Versioni del protocollo 2026-07-28, 2025-11-25, 2025-06-18 e 2025-03-26.

## TLS e certificati {#tls}

BombVault serve HTTPS con un certificato emesso da sé, e all'inizio quel certificato nomina solo `localhost`, `127.0.0.1` e `::1`. Claude Code e `mcp-remote` lo rifiutano su un indirizzo della rete locale. Le vie d'uscita, nell'ordine che va bene per la maggior parte delle installazioni Unraid:

1. **Aggiungere l'indirizzo nella scheda MCP.** Se apri la scheda in HTTPS su un indirizzo che il certificato non nomina, lo dice e offre **Aggiungi questo indirizzo al certificato**. BombVault emette di nuovo il suo certificato con quell'indirizzo (il browser avvisa ancora una volta, come la prima). Poi fai clic su **Scarica certificato**; gli snippet impostano `NODE_EXTRA_CA_CERTS` sul file scaricato, così il client si fida esattamente di quel certificato. Significa anche che ogni client configurato con un file scaricato in precedenza smette di connettersi appena il certificato viene emesso di nuovo, su questo computer e su tutti gli altri, finché non riceve il nuovo file.
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

- Senza una chiave attiva e con l'accesso tramite OAuth spento, `/mcp` risponde `404`.
- L'accesso tramite OAuth è offerto solo finché è impostata una password di accesso. Token, codici e secret dei client sono salvati solo come impronta, e un token vale solo per l'indirizzo per cui è stato emesso.
- Un client può registrarsi al massimo 10 volte all'ora dallo stesso indirizzo, e BombVault conserva al massimo 100 client registrati con cui nessuno ha effettuato l'accesso, ciascuno per un giorno. Codici e refresh token sbagliati contano per lo stesso blocco delle chiavi sbagliate.
- I consensi si comportano come le chiavi quando si ripristina un backup della configurazione o cambia `APP_KEY`: dopo un ripristino ogni client deve accedere di nuovo.
- Nessun indirizzo è esente. Le richieste da `localhost`, dall'host Unraid, da un reverse proxy o da `tailscale serve` hanno bisogno di una chiave come tutte le altre, anche quando l'interfaccia web non ha una password di accesso.
- Le chiavi sono salvate solo come impronta, mostrate una sola volta, e si possono rinominare, sostituire e revocare. Fino a 10 chiavi attive, ognuna con il proprio interruttore **Consenti di avviare backup**.
- Ogni creazione, sostituzione, modifica dei permessi e revoca invia una notifica tramite i tuoi canali di notifica, con l'indirizzo da cui è arrivata, a meno che le notifiche siano disattivate.
- 5 chiavi sbagliate al minuto per indirizzo, poi `429`. 120 richieste al minuto e 12 backup avviati all'ora per chiave, più l'attesa e la protezione della conservazione descritte sopra.
- Le richieste da una pagina del browser di un'altra origine vengono rifiutate.
- Finché non è impostata una password di accesso, non si possono creare chiavi da un nome host che sembra pubblico.
- Ogni backup avviato da un assistente, e le esecuzioni di prune e di copia off-site che ne derivano, è segnato "via MCP" con il nome della chiave nel registro attività, nel pannello degli errori e nella notifica del backup.
- Ogni chiamata a uno strumento viene scritta nel log del container con l'id della chiave e i suoi ultimi quattro caratteri (mai il nome) e contata in `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Ripristinare un backup della configurazione revoca tutte le chiavi, perché il database ripristinato può contenere chiavi che hai revocato dopo il suo salvataggio. Poi crea chiavi nuove.
- Una chiave smette di funzionare quando cambia `APP_KEY` (una reinstallazione o un ripristino su un altro container). La scheda lo rileva e segna la chiave, e **Sostituisci chiave** le ridà un segreto valido.
- Tratta una chiave come una password. Un client che non sa leggere la chiave da una variabile d'ambiente, da una richiesta o da un file della chiave la tiene in chiaro nella sua configurazione o nelle sue impostazioni, e la sua finestra lo dice. Su un computer di cui ti fidi meno, meglio una chiave di sola lettura.

## Cosa esce dalla macchina {#privacy}

Tutto ciò che un assistente legge va al fornitore di IA che sta dietro: nomi degli elementi, pianificazioni, cronologia delle esecuzioni con i messaggi di errore, id e orari dei punti di ripristino, nomi dei motori di database e dimensioni dei dump, attività in corso, dati di archiviazione, copertura e stato. BombVault toglie i percorsi dell'host, le posizioni dei repository, i nomi host, le credenziali, i comandi hook e le chiavi prima che qualcosa esca.

## Risoluzione dei problemi {#troubleshooting}

| Cosa vedi | Cosa significa |
|---|---|
| `404` | Nessuna chiave attiva e l'accesso tramite OAuth è spento, oppure il percorso è sbagliato, come `/api/mcp`. L'endpoint è `/mcp`. |
| `401` | La chiave manca, è scritta male, revocata o sostituita. Forse un proxy scarta l'intestazione `Authorization` (prova `X-API-Key`). Se la scheda segna la chiave come non più valida, `APP_KEY` è cambiata: sostituisci la chiave. |
| `403` | La richiesta è arrivata da una pagina del browser di un'altra origine. Usa un client desktop o a riga di comando. |
| `405` su GET | Normale. Il punto di connessione accetta solo `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Il client è troppo vecchio per Streamable HTTP. Aggiornalo. |
| `400` "batch requests are not accepted" | Il client invia batch JSON-RPC. Invia un messaggio per richiesta. |
| `429` | Troppe chiavi sbagliate da questo indirizzo, oppure più di 120 richieste al minuto con una chiave. Aspetta un minuto e controlla che l'assistente non sia bloccato in un ciclo. |
| Errori con "certificate", "self-signed" o "unable to verify" | Il client non si fida del certificato di BombVault. Vedi [TLS e certificati](#tls). |
| `busy` | Un altro backup o un'attività di manutenzione occupa quel dominio. Riprova quando ha finito. |
| `cooldown` | Questo elemento, questo dominio o Backup Everything è stato avviato tramite MCP meno di 15 minuti fa. |
| `retention_guard` | Un altro backup MCP lascerebbe solo punti di ripristino MCP in una finestra "conserva gli ultimi N", oppure l'elemento ha già avuto 4 backup tramite MCP nelle ultime 24 ore, contando anche quelli falliti e annullati. Nel primo caso il prossimo backup pianificato fa spazio, nel secondo l'elemento torna libero 24 ore dopo il più vecchio di quei backup. In entrambi i casi puoi avviarlo dall'interfaccia web. |
| `rate_limited` | La chiave ha usato i suoi 12 avvii di quest'ora. |
| `not_permitted` su un avvio | La chiave è di sola lettura. Attiva **Consenti di avviare backup** nella scheda; non serve riconnettersi. Su un annullamento significa che l'esecuzione non è stata avviata da questa chiave. |
| `domain_off` | Quel tipo di backup è disattivato nelle impostazioni. |
| `not_found` | BombVault non protegge quell'elemento. Aggiungilo prima nell'interfaccia web; MCP non crea mai configurazione. |
| Il client non trova il server di autorizzazione | L'accesso tramite OAuth è spento, non è impostata una password di accesso, oppure il proxy non lascia passare `/.well-known/` fino a BombVault. |
| La pagina di consenso dice che l'indirizzo di ritorno non è registrato | Il client ha inviato un indirizzo di ritorno che non ha registrato. Rimuovi il connettore nel client e aggiungilo di nuovo. |
| Un client collegato riceve `401` | Il suo consenso è stato revocato, è scaduto dopo 30 giorni senza utilizzo, oppure l'indirizzo pubblico è cambiato. Il client accede di nuovo. |

Non impostare la variabile d'ambiente `MCPGODEBUG` sul container. Cambia il comportamento della libreria MCP, e un valore malformato ferma BombVault all'avvio prima ancora che scriva una sola riga di log.
