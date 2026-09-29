# Off-site e ripristino

I backup locali ti proteggono da un container perso o da un aggiornamento andato male. La replica off-site e un kit di ripristino testato ti proteggono dall'intera macchina, dal ransomware o da un incendio. Questa pagina copre la replica off-site, il rendere quella copia a prova di manomissione, il dimostrare di poter ripristinare e il recuperare quando BombVault stesso non c'è più.

## Replica off-site

Mantieni il backup locale veloce e aggiungi una o più repliche off-site. Imposta un repo per dominio nella scheda **Impostazioni, Off-site**. BombVault vi replica i nuovi snapshot con `restic copy` su base best-effort, così un intoppo off-site non fa mai fallire il backup locale. Il repo locale resta primario.

- **Più destinazioni off-site per dominio.** Ogni dominio (container, VM, flash, config, set di file e dataset ZFS) può replicare verso più destinazioni off-site contemporaneamente, non solo una, così puoi tenere, per esempio, un rest-server sulla macchina di un amico e un bucket S3 in parallelo. Aggiungi destinazioni extra in Impostazioni, Off-site, ciascuna con il proprio repository, classe di archiviazione S3, flag append-only, conservazione e budget di crescita. Una configurazione off-site singola esistente viene riportata intatta come prima destinazione, e ogni destinazione di un dominio replica secondo il calendario off-site di quel dominio.
- **Calendario off-site per dominio** (modificato insieme a ogni altro calendario su Impostazioni, Calendari): lascialo vuoto per replicare dopo ogni backup locale, oppure imposta una cadenza (per esempio `weekly Sun 03:00`) per spedire off-site meno spesso di quanto esegui il backup localmente. Un pulsante **Replica ora** copre le esecuzioni su richiesta.
- **La conservazione off-site** risiede su Impostazioni, Off-site così puoi tenere le copie off-site più a lungo come archivio. Lascia la policy tutta a zero per non tagliare mai automaticamente gli snapshot off-site.
- **I limiti di banda** (Impostazioni, Off-site) limitano la velocità di upload/download di restic così la replica non satura la tua WAN.
- Un **indicatore di replica** mostra quale dominio sta replicando mentre è in corso (sulla sua pagina e sulla Dashboard). È un indicatore attivo, non una barra di percentuale, perché `restic copy` non espone alcun progresso leggibile da una macchina.

!!! note "Ripristina direttamente da off-site"
    Ogni browser dei backup ha un interruttore **Locale / Off-site**, così se un repo locale è perso o corrotto puoi elencare e ripristinare direttamente dalla replica off-site. L'eliminazione è per sorgente: rimuovere un backup interessa solo la copia che stai visualizzando.

## Repository primari remoti {#remote-primary-repositories}

Il percorso di backup di un dominio (Impostazioni, Percorsi e archiviazione) non si limita a una cartella locale: puntalo direttamente a un remoto restic (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:utente@host:/repo`, `rclone:remoto:bucket/percorso`) e BombVault salva lì direttamente, senza copia locale separata e senza passo di replica. È una forma davvero diversa dalla replica off-site vista sopra: là il repository locale è il primario e quello off-site ne è un archivio per quanto possibile; qui il repository remoto **è** il primario, ed è l'unica copia finché non configuri anche una replica off-site (o un secondo remoto) per quel dominio.

Ognuno dei sei campi di percorso (Contenitori, Macchine virtuali, Flash, Configurazione, File, Dataset ZFS) ha subito accanto un interruttore **Locale / Remoto**:

- **Locale** mostra il consueto sfoglia-cartelle.
- **Remoto** lo sostituisce con un semplice campo URL, più un pulsante che apre la stessa finestra di test connessione e credenziali usata dalle destinazioni off-site, configurata però per questo primario. Da lì ottieni:
    - **Un test di connessione** contro il percorso reale, prima di farci affidamento.
    - **Limiti di banda** (invio e ricezione), perché un backup pianificato verso un primario remoto non saturi la tua linea WAN: le stesse opzioni restic `--limit-upload` e `--limit-download` usate dalla replica off-site, applicate al backup stesso.
    - **Protezione append-only (immutabilità)**, verificata con lo stesso test di manomissione attivo (una vera sonda DELETE verso l'altro capo) che ricevono le destinazioni off-site. Con essa attiva, BombVault si rifiuta di potare il repository: poiché dietro non c'è una copia locale separata, le credenziali su questa macchina non devono poter cancellare l'unica copia del backup.
    - **Un allarme sul budget di crescita**, ricavato dallo stesso andamento della dimensione del repository che la scheda Archiviazione già segue.

Niente di tutto questo è obbligatorio: un percorso remoto scritto a mano senza impostazioni di sicurezza salvate esegue il backup esattamente come sempre (banda illimitata, potabile, nessun allarme di budget). La finestra di sicurezza serve per quando vuoi le stesse protezioni che ottiene una copia off-site, senza dover creare una destinazione off-site solo per quello.

!!! note "Le credenziali cloud e REST sono condivise"
    Un primario remoto si autentica con le stesse credenziali S3/REST configurate in Impostazioni, Off-site, Credenziali cloud: non esiste un archivio di credenziali separato per i repository primari.

## Off-site immutabile (append-only)

Contrassegna un repo off-site come append-only così ransomware, o un host compromesso, non possano eliminare o riscrivere i tuoi backup. L'altro lato (un `restic/rest-server` in esecuzione in modalità `--append-only`) lo **impone**. BombVault lo **verifica** soltanto e non mostra mai verde sulla sola affermazione di una configurazione.

La procedura guidata di **configurazione off-site guidata** ti accompagna dalla scelta del backend (rest-server / rclone / S3) attraverso uno snippet di deploy del rest-server pronto da incollare, un test di connessione, l'interruttore immutabile (che esegue immediatamente il tamper test) e una strategia di conservazione, così l'off-site append-only è raggiungibile senza modificare a mano le configurazioni.

!!! note "Una cancellazione riuscita sotto `/locks/` è prevista"
    Append-only non significa che non si possa più cancellare nulla. restic deve prendere e rilasciare i propri lock, quindi `/locks/` resta scrivibile e cancellabile di proposito. Gli snapshot e i dati che stanno dietro, cioè proprio ciò che un ransomware cercherebbe, non possono essere rimossi. Se sondi tu stesso il lato remoto, una cancellazione che riesce sotto `/locks/` è il comportamento corretto e non una falla.

!!! warning "I repo immutabili non vengono mai potati da questa macchina"
    Un off-site immutabile deliberatamente non pota mai i vecchi snapshot. Impostagli un **allarme del budget di crescita** così vieni avvisato prima che la dimensione del repo sfugga di mano.

## Tamper test

BombVault dimostra periodicamente la garanzia append-only tentando effettivamente un'eliminazione contro il repo off-site, mirata a un oggetto inesistente:

- **Rifiutata** significa protetto.
- **Accettata** significa non protetto.
- Un risultato **inconcludente** (server irraggiungibile, errore di autenticazione) non ribalta mai il verdetto memorizzato.

Un reale passaggio da protetto a non protetto fa scattare un unico avviso.

## Esercitazioni DR

BombVault offre due livelli di prova che i tuoi backup siano effettivamente ripristinabili, non solo presenti.

- **Esercitazioni di verifica del ripristino (locali).** BombVault esegue periodicamente `restic check --read-data-subset` (limitato, mai un ripristino completo che riempie il disco) e mostra un badge *ultima ripristinabilità verificata* per dominio. La cadenza risiede su Impostazioni, Calendari; il badge su Impostazioni, Integrità.
- **Esercitazioni DR (off-site).** BombVault ripristina una destinazione reale dal repo off-site in una sandbox usa e getta, la verifica file per file e byte per byte, poi ripulisce. Questo dimostra che puoi recuperare da off-site, non solo che il repo risponde.

La **scorecard della protezione dal ransomware** sulla Dashboard riassume tutto questo in una postura verde / ambra / rossa per dominio, con una checklist con marca temporale (off-site configurato, append-only verificato, replica aggiornata, esercitazione di ripristino superata, cifratura attiva, strategia di pota impostata). Ogni riga rossa collega direttamente alla soluzione, e la scheda diventa verde solo su fatti verificati.

## Associazione delle istanze {#pairing}

I Riceventi, le fonti di Prelievo, la vista Flotta e il Mesh fuori sede parlano tutti con un altro BombVault. Lo fanno come membri di un unico gruppo di associazione, e un'istanza si unisce al gruppo con dodici parole.

Sulla prima istanza apri **Istanze → Associazione** e premi **Genera frase**. Appaiono dodici parole. Su ogni altra istanza apri la stessa scheda, premi **Inserisci frase** e incollale o digitale. Una parola che non è nell'elenco viene indicata con la sua posizione già mentre la digiti, e l'ultima parola porta un checksum, così una parola digitata male o scambiata viene rilevata prima che avvenga l'associazione. Genera la frase su una sola istanza: due istanze che creano entrambe una frase formano due gruppi separati. Se per un minuto non si presenta nessuno, la scheda indica come lasciare il gruppo in eccesso e unirsi all'altro. L'associazione richiede una password di accesso su ogni istanza, perché le parole aprono i backup di tutte le istanze del gruppo. La frase può essere mostrata di nuovo in seguito, dopo aver inserito quella password. **Esci dal gruppo** fa uscire di nuovo un'istanza.

Chiunque conosca le parole può unirsi al gruppo, trattale quindi come una password.

**Come i membri si raggiungono.** Ogni istanza apprende il proprio indirizzo sulla rete dal tuo browser nel momento in cui accedi, mostrato nella scheda del relay come **Questa istanza sulla tua rete**; correggilo lì se davanti c'è un reverse proxy o una porta insolita. Sulla stessa rete i membri annunciano quell'indirizzo tramite multicast e si parlano direttamente, e dove il multicast non riesce ad attraversare una rete di contenitori, come la rete bridge predefinita di Docker, un'istanza cerca invece le altre nella propria subnet con una chiamata firmata a cui solo un membro del gruppo sa rispondere, così l'associazione si completa comunque in pochi secondi senza relay. Se non salta fuori nulla, **Non la trovi?**, sotto la scheda di associazione, accetta un indirizzo inserito a mano, per un'altra subnet o una porta non standard. Le istanze su reti diverse passano da un relay, scelto nella stessa scheda:

- **Relay del progetto** (predefinito): `relay.halleluja.design`, lo stesso relay usato anche da KnightLoader. Niente da configurare.
- **Relay personale**: il container **BombVault Relay** dalle Unraid Community Apps, oppure una delle tue istanze già raggiungibile dall'esterno con **Servi da relay** attivato. Quell'istanza risponde poi su `/relay/connect` al proprio indirizzo, dietro il reverse proxy e il certificato che già possiede, e fa entrare solo il tuo gruppo. Inserisci l'indirizzo del relay su ogni istanza che deve usarlo.
- **Nessun relay**: i membri si trovano automaticamente solo sulla stessa rete, e in nessun altro posto.

**Cosa vede il relay.** Ogni chiamata tra membri è sigillata con AES-256-GCM sotto una chiave derivata dalle dodici parole, e quella chiave non lascia mai le tue istanze. Il relay apprende un hash che raggruppa le connessioni, a quale istanza è destinato un messaggio, quanto è grande e quando passa. Una chiamata diretta sulla rete locale è sigillata allo stesso modo e anche firmata, così nulla dipende dal certificato autofirmato che un'istanza serve.

**Cosa viaggia sul gruppo.** La Scorecard della Flotta, una richiesta di controllare subito un dominio, le offerte di storage fuori sede del Mesh, e ciò di cui un Ricevente o una fonte di Prelievo ha bisogno: le posizioni del repository dell'altra istanza e la sua password restic. I dati di backup invece mai: vanno sempre direttamente ai backend restic. Nemmeno l'APP_KEY: la password restic apre i repository di quell'istanza e nient'altro, non i suoi segreti memorizzati, non le sue sessioni, non i suoi codici di ripristino.

**Voci precedenti all'associazione.** I peer di Flotta aggiunti con un token di flotta, e i Riceventi e le fonti di Prelievo configurati con l'APP_KEY dell'altra istanza, restano dopo l'aggiornamento e sono contrassegnati **Associa di nuovo**. I Riceventi e le fonti di Prelievo continuano a funzionare: al primo avvio BombVault sostituisce ogni APP_KEY memorizzato con la password restic da esso derivata. Associa entrambe le istanze, poi modifica la voce e scegli la sua istanza. Un peer di Flotta riprende la sua vecchia riga non appena un'istanza con lo stesso nome compare nel gruppo.

L'unico posto che richiede ancora un APP_KEY a mano è [Ripristino da un altro repo BombVault](#restore-from-another-bombvault-repo), per il caso in cui l'altra istanza sia scomparsa e non possa rispondere in un gruppo.

## Dashboard ricevente (il lato ricevente)

![Il lato ricevente, osservato in sola lettura, con un controllo di integrità eseguito su questa macchina.](assets/screenshots/receiver.png)

*Il lato ricevente, osservato in sola lettura, con un controllo di integrità eseguito su questa macchina.*

Tutto quanto sopra è il lato *mittente*. Sulla macchina che **riceve** copie off-site immutabili da un altro BombVault, la dashboard Ricevente ti offre un monitoraggio indipendente e in sola lettura di quei repository sull'hardware ricevente, così un fallimento silenzioso all'altra estremità non passa inosservato.

Attiva l'interruttore **Ricevente** in Impostazioni per rivelare una scheda **Ricevente**. È disattivato di default; abilitalo solo su una macchina che riceve effettivamente backup off-site immutabili. Poi registra un repository ricevuto (in sola lettura, aperto con la password restic dell'istanza mittente, che ottiene tramite il [gruppo di associazione](#pairing)) per ottenere:

- **Un inventario di snapshot raggruppato per sorgente**, così puoi vedere esattamente quali container, VM e set di file sono arrivati.
- **Ultimo ricevuto** per sorgente, così sai quanto è fresco ciascuno.
- **Un `restic check` indipendente** eseguito sull'hardware ricevente, così l'integrità viene verificata dove i dati effettivamente risiedono, non solo sul mittente.
- **Un dead-man's switch:** un avviso quando una sorgente smette di inviare entro una finestra che imposti.
- **Avvisi di integrità:** un avviso quando un controllo sul lato ricevente fallisce.

Il Ricevente è rigorosamente in sola lettura. Non scrive mai nel repository ricevuto, così non può mai rompere la garanzia append-only su cui il mittente fa affidamento.

## Esempio completo: due macchine Unraid, dall'inizio alla fine

Sopra sono descritti i pezzi. Questa è un'installazione completa con valori reali, perché i pezzi si montano meglio dopo averli visti montati una volta.

Due macchine: **TOWER** esegue i container e invia i backup, **VAULT** li riceve e impone l'immutabilità. Sostituisci con i tuoi nomi, indirizzi e percorsi di condivisione.

**1. Su VAULT, avvia il server append-only.** In BombVault su TOWER vai su *Impostazioni → Off-site → configurazione guidata*, scegli **rest-server** e genera la ricetta. Copia la scheda **Modello Unraid (XML)**, salvala su VAULT come `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, poi *Docker → Add Container* e scegli **rest-server** dall'elenco dei modelli. Prima di avviarlo, scrivi la riga `htpasswd` mostrata in `/mnt/user/appdata/rest-server/.htpasswd` su VAULT. La password monouso viene mostrata una sola volta e non viene mai conservata: copiala ora. Quella riga porta la stessa password, già cifrata con bcrypt per te: il testo in chiaro va nelle credenziali REST su TOWER, la riga cifrata nel `.htpasswd` su VAULT. Non devi cifrare nulla tu.

    Lascia `--append-only` nel campo OPTIONS. È tutto il senso della cosa: senza, VAULT torna a essere una normale condivisione.

**2. Su TOWER, punta il repository off-site su di esso.** L'URL del repository segue lo schema stampato dalla ricetta:

    rest:http://VAULT:8000/bombvault-containers/containers

Il primo segmento del percorso è l'utente htpasswd, il secondo il repository. Inserisci l'utente e la password generati come credenziali REST della destinazione, poi esegui il **test di connessione**.

**3. Su TOWER, attiva «Immutabile».** Il test di manomissione parte subito e deve dire *protetto*. Cosa significano le risposte:

| Risultato | Cosa è successo |
| --- | --- |
| **protetto** | VAULT ha rifiutato la cancellazione. È l'unico stato che passa. |
| **NON protetto** | VAULT ha accettato una cancellazione. Manca `--append-only` oppure è stato rimosso. |
| **non conclusivo** | Né l'uno né l'altro. Di solito l'URL non è quello che usa restic, oppure le credenziali sono cambiate. Non viene registrato nulla e non scatta alcun avviso. |

**4. Su VAULT, guarda cosa arriva.** Associa le due macchine ([Associazione delle istanze](#pairing)), attiva *Impostazioni → Ricevitore*, apri la scheda **Ricevitore** e registra il repository in sola lettura con TOWER come istanza mittente.

!!! warning "La posizione è un percorso **dentro** il container, scritto relativo al mount dell'host"
    Inserisci `user/appdata/rest-server/bombvault-containers/containers`, **non** `/mnt/user/appdata/…`. BombVault gira in un container in cui il `/mnt` dell'host è montato altrove; un percorso host assoluto lì non esiste. Se ne incolli uno, BombVault ora ti indica il percorso relativo da usare.

    VAULT ottiene la password restic di TOWER tramite il gruppo quando salvi; nessuno deve digitare una chiave.

**5. Rendilo reciproco, se vuoi.** Ripeti gli stessi cinque passi nella direzione opposta: un rest-server su TOWER che riceve la copia di VAULT. Ogni macchina impone allora l'immutabilità all'altra, e nessuna può cancellare i backup dell'altra.

## Ripristino guidato

Una scheda **Ripristino** dedicata accompagna un'installazione pulita o ricostruita attraverso il caso di disastro, in un unico posto:

1. **Ripristina prima le impostazioni di BombVault stesso**, così i percorsi di backup, le destinazioni off-site e le credenziali di cui il resto del flusso ha bisogno risultano precompilati (applicato tramite un auto-riavvio sul socket Docker, così il database delle impostazioni in esecuzione non viene mai sovrascritto sotto un handle aperto).
2. **Verifica che BombVault possa leggere i tuoi backup** (l'insidia della chiave di crittografia messa in primo piano).
3. Ti permette di **puntare al tuo repo esistente** (locale o off-site).
4. **Scopre** i container, le VM, i set di file e i dataset ZFS memorizzati al suo interno.
5. **Ripristina container e VM in un colpo solo** (lasciati fermi, così li avvii deliberatamente) ed elenca i set di file e gli elementi ZFS da ripristinare uno alla volta; gli elementi ZFS tornano disattivati. Il tuo kit di ripristino è a un clic di distanza.

!!! tip "Migrazione pianificata contro disastro"
    Il ripristino guidato ripristina le impostazioni di BombVault stesso da un backup. Per uno spostamento *pianificato* su una nuova macchina, puoi invece portare la tua configurazione direttamente con la scheda **Esporta e importa impostazioni** (un file JSON portatile). Vedi [Configurazione](configuration.md#portable-settings-export-and-import).

### Ripristino da un altro repo BombVault {#restore-from-another-bombvault-repo}

Una scheda separata nella scheda **Ripristino** apre il repo di un'*altra* istanza BombVault (una condivisione montata sotto `/mnt`, o un URL remoto) con l'**`APP_KEY` di quell'istanza**, in una sessione monouso e in sola lettura. Sfoglia i container, le VM e i set di file memorizzati lì, scegli uno snapshot e ripristinalo, e l'oggetto ripristinato diventa un normale container, VM o set di file locale. Nulla viene mai scritto nell'altro repo, e le tue impostazioni di backup restano intatte (la sessione risiede in memoria e scade da sé). Spostare un container dal server A al server B non significa più ripuntare le impostazioni del tuo repo e riportarle indietro dopo. La federazione dal vivo server-a-server è esplicitamente fuori ambito; questa è una deliberata estrazione monouso.

## Kit di ripristino della chiave di crittografia

Questo è il pezzo che rende possibile il disaster recovery anche quando non c'è alcun BombVault in esecuzione.

Un clic scarica la **chiave master**, la **password restic derivata** e le **posizioni e comandi esatti del repo**, così puoi ripristinare direttamente con la CLI di restic su qualsiasi macchina. Un promemoria sulla Dashboard ti assilla finché non l'hai conservato.

!!! danger "Conserva il kit di ripristino fuori dal server"
    Il kit contiene il segreto che decifra i tuoi backup. Tienilo in un luogo sicuro e separato dal server (un password manager, una copia stampata in una cassaforte). Se perdi sia BombVault che `APP_KEY` senza kit di ripristino, i tuoi backup cifrati non possono essere recuperati.

!!! warning "Lo snapshot più recente non è sempre quello da ripristinare"
    Da restic 0.17, `restic snapshots` mostra la dimensione di ogni snapshot. Dopo una perdita di dati lo snapshot più recente può essere quello svuotato, quindi non ripristinare uno snapshot molto più piccolo dei precedenti. Dopo un ransomware può essere quello cifrato, di dimensione normale. Se BombVault funziona ancora, controlla prima la sua pagina **Anomalie**: indica l'ultimo backup buono. Un ripristino non ha bisogno di alcun dato sulle anomalie di BombVault, e la pausa della conservazione mantiene sempre solo più snapshot.

### Se il kit non è a portata di mano

La password non è memorizzata da nessuna parte, viene **calcolata** dall'`APP_KEY`. Con la chiave e una shell puoi quindi riprodurla da solo:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

È un HMAC-SHA256 sulla stringa fissa `bombvault:restic-repo`, con i byte grezzi dell'`APP_KEY` esadecimale come chiave, stampato come 64 caratteri esadecimali minuscoli. Lo stesso valore è nel kit, come password restic derivata; questo serve per il giorno in cui il kit si trova altrove rispetto a te.

!!! warning "Per un repository ricevuto, usa la chiave dell'istanza MITTENTE"
    Un repository arrivato qui tramite replica off-site è stato creato dalla macchina che lo ha inviato, con la **sua** `APP_KEY`. Derivare dalla chiave della macchina ricevente produce una password che restic rifiuta, il che sembra esattamente un repository corrotto senza esserlo. È il motivo abituale per cui `restic check` su un repository ricevuto continua a chiedere la password.

Poiché le definizioni di ripristino risiedono **dentro** ogni repo (`<repo>/def`, `<repo>/vm-def`), una cartella di repo copiata è completamente autonoma, così il kit più il repo sono tutto ciò che serve per un ripristino bare-metal.

## Recuperare un dump del database {#database-dumps}

Un dump del database è un punto di ripristino a sé nel repository dei container, con l'etichetta `dbdump:<container>` e un solo file, `/dbdump/<container>.sql`. BombVault li elenca, li scarica e li importa sotto **Backup**; qui sotto ci sono gli stessi passi con il solo restic, per il giorno in cui BombVault non c'è.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Le etichette `dbversion:` e `dbname:` su ogni dump dicono da quale versione del server proviene e quali database contiene. Un file completo termina con `-- PostgreSQL database cluster dump complete` oppure `-- Dump completed`.

Importalo in un container della stessa versione o di una più recente (PostgreSQL), o della stessa versione maggiore (MySQL e MariaDB), avviato una volta con la cartella dati vuota perché si inizializzi. L'host non ha bisogno di un client del database, il container ne ha uno:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Per un singolo database dentro un dump completo, MySQL e MariaDB accettano `--one-database <name>` sul comando del client. Un dump PostgreSQL ha una sezione per database, ciascuna aperta da una riga `\connect <name>`: copia quella sezione in un file a parte e importalo con `-d <name>` dopo aver creato il database.

!!! warning "Un dump preso come root porta con sé gli utenti del server"
    Un dump completo di MySQL o MariaDB preso come root contiene il database di sistema `mysql`, quindi importarlo sostituisce gli account del nuovo server, password di root compresa, con quelli del dump. Su PostgreSQL, `role ... already exists` per l'utente creato dal container è previsto e innocuo.
