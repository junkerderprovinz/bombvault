# Nodi

Un nodo è un BombVault senza interfaccia web, per esempio su un VPS o su un piccolo server a casa di un amico. Fa il backup dei container e delle cartelle del proprio server, e il BombVault su cui lo aggiungi, che in questa pagina chiamiamo **istanza madre**, gli dice quando fare il backup e dove mandarlo. Lo gestisci dall'interfaccia web dell'istanza madre, accanto ai tuoi elementi.

![La scheda del nodo vps-falkenstein nella pagina Istanze: Connessa, la sua versione, i suoi backup sul server ricevente, quando fa il backup ogni sezione, e Cambia destinazione, Sostituisci e Rilascia.](assets/screenshots/nodes/card-dark.png#only-dark)
![La scheda del nodo vps-falkenstein nella pagina Istanze: Connessa, la sua versione, i suoi backup sul server ricevente, quando fa il backup ogni sezione, e Cambia destinazione, Sostituisci e Rilascia.](assets/screenshots/nodes/card-light.png#only-light)

## Come funziona {#how-it-works}

- **L'istanza madre custodisce la chiave del nodo.** Quando aggiungi un nodo, l'istanza madre crea la sua `APP_KEY`, ne tiene una copia sigillata e te la consegna una sola volta, nel file compose del nodo. Con quella copia l'istanza madre raggiunge il nodo e apre i suoi backup.
- **Il nodo non ha bisogno di porte aperte.** L'istanza madre e il nodo parlano attraverso il relay su un collegamento privato tutto loro, separato dal tuo gruppo di associazione. L'istanza madre avvia ogni chiamata e la firma, e il nodo non le chiede nulla se non il suo saluto. L'unica destinazione che richiede una porta aperta è [tenere i backup sul nodo](#keep-on-node).
- **I backup non passano mai dal relay.** Il nodo li manda direttamente nel posto che hai scelto: il server ricevente dell'istanza madre, una delle sue destinazioni o un rest-server sul nodo stesso.
- **Il nodo non pianifica nulla da solo.** Non ha pianificazioni, non manda notifiche e non entra in nessun gruppo né in nessuna fortezza. L'istanza madre avvia i suoi backup, controlla come vanno e manda gli avvisi.
- **Un nodo fa il backup di container e set di cartelle.** VM, chiavetta flash, dataset ZFS e auto-backup restano alle installazioni complete.

Il collegamento usa il relay dell'istanza madre quando il suo gruppo ne ha uno, altrimenti il relay del progetto, anche quando l'istanza madre si associa senza relay.

## Prima di iniziare {#before-you-start}

- **Una password di accesso sull'istanza madre.** Aggiungere un nodo, mostrare di nuovo il suo file compose e aprire i suoi elementi richiedono un accesso con la password, vedi [Accesso](configuration.md#login).
- **Docker con Compose sul server del nodo.**
- **Un posto per i backup**, uno dei tre descritti in [Dove vanno i backup](#placement). Il server ricevente è la scelta più sicura, perché lì il nodo non può eliminare nulla.

## Aggiungere un nodo {#add}

1. Sull'istanza madre, apri **Istanze** e scegli **Aggiungi nodo** in basso a destra.
2. **Nome.** Scrivi come il nodo deve comparire qui, per esempio il nome del server, e scegli **Avanti**.

    ![La finestra Aggiungi nodo al primo passo, con il nome vps-helsinki inserito.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![La finestra Aggiungi nodo al primo passo, con il nome vps-helsinki inserito.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Backup.** Scegli dove vanno i backup e compila ciò che quel posto richiede. **Fa il backup ogni giorno alle** è l'ora in cui partono entrambe le sezioni. L'istanza madre propone mezz'ora dopo i propri backup dei container, così i due non usano gli stessi dischi nello stesso momento. Più avanti puoi dare a ogni sezione una propria pianificazione dalla scheda. Scegli **Avanti**.

    ![Il secondo passo: il server ricevente scelto, Indirizzo per i nodi compilato con un indirizzo https e Fa il backup ogni giorno alle 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Il secondo passo: il server ricevente scelto, Indirizzo per i nodi compilato con un indirizzo https e Fa il backup ogni giorno alle 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Avvio.** La finestra mostra il file compose del nodo con `MODE`, `APP_KEY`, `NODE_HOME` e `NODE_NAME` già compilati. Copialo e conserva la `APP_KEY` in un posto sicuro. Sul server del nodo:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Prima di avviarlo, punta il volume Host Data sui dati di cui vuoi il backup e imposta `HOST_SOURCE_ROOT` sullo stesso percorso, come mostra l'esempio qui sotto.

5. La finestra aspetta il nodo. Appena il nodo risponde, l'istanza madre gli dice dove vanno i suoi backup e controlla che li raggiunga, e la finestra dice **vps-helsinki è online e prende ordini da questa istanza.**

    ![L'ultimo passo: il file compose, poi il nodo online e i suoi backup sul server ricevente.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![L'ultimo passo: il file compose, poi il nodo online e i suoi backup sul server ricevente.](assets/screenshots/nodes/add-start-light.png#only-light)

Puoi chiudere la finestra prima che il nodo risponda. La sua scheda dice allora **In attesa del nodo**, e l'istanza madre gli comunica dove vanno i suoi backup appena si fa vivo.

### Esempio: un VPS che fa il backup sul server ricevente {#example-vps}

L'istanza madre è una macchina Unraid chiamata `tower` che gestisce il [server ricevente](offsite-recovery.md#receiving-server). Un reverse proxy lo pubblica come `https://backup.tower.example.org`. Il VPS `vps-helsinki` fa girare Nextcloud e Caddy, con i loro dati in `/srv/appdata`.

1. Su `tower`: **Istanze**, **Aggiungi nodo**, nome `vps-helsinki`, **Avanti**.
2. Scegli **Il server ricevente di questa istanza** e scrivi `https://backup.tower.example.org` in **Indirizzo per i nodi**. I container dell'istanza madre fanno il backup alle 02:30, quindi la finestra propone le 03:00. Lascia così e scegli **Avanti**.
3. Sul VPS, salva il file della finestra con i volumi e `HOST_SOURCE_ROOT` cambiati per `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Pochi secondi dopo, la finestra su `tower` dice che il nodo è online e **I backup vanno a https://backup.tower.example.org.**
5. Alle 03:00 l'istanza madre avvia i backup del nodo. La mattina dopo, la scheda **Nodi** nella panoramica mostra l'ultimo backup di `vps-helsinki`, e **Backup** elenca i suoi container in una sezione a parte.

Come BombVault trova i dati di un container su un semplice host Docker è descritto in [Rilevamento delle sorgenti di backup](configuration.md#backup-source-detection).

## Dove vanno i backup {#placement}

Ogni nodo ha una sola destinazione per entrambe le sezioni. **Cambia destinazione** sulla sua scheda la cambia appena il nodo è online. Il nuovo posto riceve nuovi repository, e i backup nel posto precedente restano dove sono.

### Il server ricevente di questa istanza {#to-receiver}

Il nodo scrive sul server ricevente dell'istanza madre in modalità append-only: può aggiungere backup ma non eliminarne nessuno. Lì riceve un accesso proprio, col nome del nodo, e un repository per sezione nella propria cartella. L'istanza madre aggiunge la propria chiave a ogni repository e li pota da sé, visto che il nodo non può farlo.

Il nodo raggiunge il server all'**Indirizzo per i nodi**, che imposti nel secondo passo o nella finestra **Server ricevente**. Di solito un nodo invia da internet, quindi l'indirizzo deve essere uno di questi:

- `https://`, per esempio dietro un reverse proxy, Pangolin o un tunnel. Se il server mostra un certificato proprio invece di uno di un'autorità pubblica, incollalo in formato PEM in **Certificato**.
- `http://` con un indirizzo IP privato, come `10.0.0.5` su un collegamento WireGuard o `100.101.102.103` su Tailscale.

Un semplice `http://` verso un indirizzo pubblico viene rifiutato, perché il nodo manderebbe i suoi backup attraverso internet senza cifratura. Quando cambi l'indirizzo, l'istanza madre lo comunica a ogni nodo del ricevitore, e un nodo assente lo viene a sapere appena torna.

### Una destinazione {#to-location}

Il nodo scrive su una delle destinazioni dell'istanza madre che riesce a raggiungere da solo: un servizio S3 o un server REST, su `https://` o a un indirizzo privato. I suoi repository stanno lì in `bombvault-node-<node id>-containers` e `bombvault-node-<node id>-files`.

Il nodo accede con delle [credenziali salvate](offsite-recovery.md#sign-ins) create apposta per lui, che devono essere diverse da quelle con cui l'istanza madre scrive sulla destinazione. Se la finestra dice che non ce ne sono, aggiungine prima di tutto. Con queste credenziali il nodo di solito può eliminare ciò che si trova sulla destinazione, a differenza del server ricevente, e la finestra lo segnala.

### Tenerli sul nodo {#keep-on-node}

Il nodo tiene i backup da sé, dietro un proprio rest-server in modalità append-only, e l'istanza madre preleva una copia di ogni sezione nel proprio spazio. Va bene per un nodo a cui l'istanza madre non può offrire spazio. È l'unica destinazione che richiede una porta aperta.

![Il secondo passo con Tenerli sul nodo scelto, l'indirizzo https://203.0.113.7:8443 e la nota sulla porta aperta.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Il secondo passo con Tenerli sul nodo scelto, l'indirizzo https://203.0.113.7:8443 e la nota sulla porta aperta.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. In **Indirizzo del nodo**, scrivi dove l'istanza madre raggiunge il nodo: il suo indirizzo pubblico e una porta libera, come `https://203.0.113.7:8443`, senza cartella dopo. Usa `http://` solo con un indirizzo privato, come sopra.
2. Il nodo avvia il container `bombvault-node-rest-server` su quella porta, con i suoi repository nella cartella `bombvault-node` dentro Host Data. Con `https://` crea un certificato per quell'indirizzo, e l'istanza madre lo fissa.
3. Apri la porta nel firewall del nodo, così l'istanza madre può raggiungerla. Una porta già occupata da un altro container sul nodo viene rifiutata con il nome di quel container.

L'istanza madre preleva una sezione subito dopo la fine dei backup del nodo per quella sezione, e come riserva ogni ora e un quarto, anche con **Prelievo** disattivato. Ciò che l'istanza madre ha prelevato resta presso di lei come ogni [copia prelevata](offsite-recovery.md#receiver), anche dopo che il nodo non c'è più. Il nodo pota i propri repository quando l'istanza madre glielo chiede, perché il suo rest-server non accetta eliminazioni, e i suoi set di cartelle lasciano fuori la cartella `bombvault-node`, così non fa mai il backup dei propri backup.

## Quando fanno il backup i nodi {#schedule}

L'istanza madre gestisce la pianificazione di ogni nodo. La scheda mostra un piano per sezione, come **Container** *Ogni giorno alle 2:00* e **Cartelle** *Settimanale (dom) alle 4:00*. Un clic apre lo stesso editor di [Pianificazioni](schedules.md#plans), e una modifica viene salvata subito.

![Il piano dei container di un nodo aperto dalla sua scheda, con Disattivato, Giornaliero, Settimanale, Ogni N giorni e Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Il piano dei container di un nodo aperto dalla sua scheda, con Disattivato, Giornaliero, Settimanale, Ogni N giorni e Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Un nodo che era assente** avvia i backup persi appena torna. L'istanza madre controlla ogni cinque minuti.
- **A un nodo occupato** viene chiesto di nuovo cinque minuti dopo.
- **Mentre l'istanza madre pota** i vecchi backup di un nodo, il nodo non avvia nulla finché lei non ha finito. Sul server ricevente e sul nodo stesso questa finestra si apre ogni domenica alle 03:00 e dura fino a sei ore. Su una destinazione segue la pianificazione della copia off-site dell'istanza madre per quella destinazione. L'istanza madre conserva i backup secondo la propria regola per la sezione in **Per quanto tempo conservare**, oppure secondo la regola della destinazione, se ne ha una.
- **Nell'elenco delle esecuzioni del nodo**, i backup e i ripristini avviati dall'istanza madre compaiono come avviati da lei.

## Tenere d'occhio i nodi {#watch}

La scheda **Nodi** nella panoramica elenca ogni nodo con il suo stato, l'ultimo backup, gli errori e le anomalie aperte. Compare appena hai un nodo.

![La scheda Nodi nella panoramica con tre nodi: uno connesso, uno inattivo con due errori e uno ancora in attesa del nodo.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![La scheda Nodi nella panoramica con tre nodi: uno connesso, uno inattivo con due errori e uno ancora in attesa del nodo.](assets/screenshots/nodes/overview-card-light.png#only-light)

L'istanza madre manda un avviso per episodio attraverso i suoi [canali di notifica](notifications.md), e il successivo solo quando quell'episodio è finito:

- **Un backup non è riuscito**, per esempio *BombVault: vps-helsinki: backup failed*, con l'elemento e il motivo. Un backup successivo di quell'elemento che va a buon fine chiude l'episodio.
- **Il nodo non è raggiungibile**, per esempio *BombVault: vps-helsinki is out of reach*. L'istanza madre aspetta quanto l'interruttore dell'uomo morto più breve tra i repository del nodo, 26 ore se non lo cambi, oppure 6 ore finché il nodo non ha ancora una destinazione.

Il monitoraggio del ricevitore dell'istanza madre si occupa anche di ogni repository di un nodo, con un interruttore dell'uomo morto e un `restic check` giornaliero alle 04:00, vedi [Ricevitore e prelevatore](offsite-recovery.md#receiver).

## Gli elementi di un nodo {#entries}

In **Backup**, gli elenchi Container e Cartelle hanno una sezione per ogni nodo sotto i tuoi elementi. Il container del nodo stesso non compare. **Esegui backup ora** nel menu di un elemento avvia il suo backup sul nodo, e **Apri** apre la sua pagina.

Nella sezione di un nodo dell'elenco Cartelle, **Aggiungi set di cartelle** aggiunge un set di cartelle sul nodo, e **Modifica** nel menu di un set lo cambia. Entrambi aprono la stessa finestra dei tuoi set. Il selettore di cartelle della finestra sfoglia le cartelle del nodo, ma non vi crea cartelle nuove.

![La sezione del nodo vps-falkenstein nell'elenco Container: nextcloud, nextcloud-db, caddy e uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![La sezione del nodo vps-falkenstein nell'elenco Container: nextcloud, nextcloud-db, caddy e uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

La pagina di un elemento di un nodo dice **Su** e il nome del nodo, ed elenca i suoi backup. **Ripristina** su uno di essi ripristina l'elemento sul nodo e sostituisce ciò che c'è adesso. Per farlo il container viene fermato, come sull'istanza madre.

![La pagina di nextcloud su vps-falkenstein con i suoi backup e un pulsante Ripristina accanto a ciascuno.](assets/screenshots/nodes/entry-dark.png#only-dark)
![La pagina di nextcloud su vps-falkenstein con i suoi backup e un pulsante Ripristina accanto a ciascuno.](assets/screenshots/nodes/entry-light.png#only-light)

Attraverso l'istanza madre raggiungi un elenco fisso di percorsi del nodo: i suoi container, set di cartelle, esecuzioni e anomalie, e il loro backup e ripristino. Le impostazioni, le chiavi e lo spazio di archiviazione del nodo restano fuori portata, e nulla in quell'elenco elimina. Vedi [I nodi nell'API](api.md#nodes).

## Sostituire e rilasciare {#replace-release}

**Sostituisci** mostra di nuovo il file compose del nodo, dopo un accesso con la password. Usalo per spostare il nodo su un altro server o per ricostruirlo: avvia il file lì e spegni il vecchio server. La stessa `APP_KEY` e lo stesso `NODE_HOME` lo rendono di nuovo lo stesso nodo, con i suoi backup e la sua cronologia. Due server con la stessa `APP_KEY` contano come un solo nodo, quindi non farli mai girare entrambi.

**Rilascia** lascia andare un nodo. Smette di prendere ordini dall'istanza madre e non fa più nessun backup. I suoi backup restano dove sono, come ogni copia che l'istanza madre ha prelevato. Il suo accesso sul server ricevente sparisce, e l'istanza madre smette di controllare i suoi repository. Un nodo assente al momento del rilascio mantiene la sua scheda come **In fase di rilascio** finché non lo viene a sapere. Un nodo rilasciato non accetta più un'istanza madre con la stessa `APP_KEY`. Per usare di nuovo il server, aggiungilo come nuovo nodo e avvialo con il nuovo file compose e una cartella `config` vuota.

## Sicurezza {#security}

Se qualcuno prende il controllo dell'istanza madre, controlla anche i suoi nodi, e un nodo ha in mano Docker sul suo server, cioè root. Proteggi l'istanza madre con una password e tienila lontana da internet. Se qualcuno prende il controllo di un nodo, non ottiene nessun gruppo di associazione, nessuna fortezza, nessun altro nodo e nessun altro repository, e non può cancellare la cronologia sul server ricevente. I dettagli sono in [I nodi nel modello di sicurezza](configuration.md#nodes).

Il [kit di ripristino](offsite-recovery.md#recovery-kit) ha un capitolo **Nodes** con la `APP_KEY` di ogni nodo, la sua password restic e i suoi repository, così puoi riportare in vita un nodo o ripristinarne i backup con il solo restic.
