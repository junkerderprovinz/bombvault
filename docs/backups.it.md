# Backup ed elementi

Tutto ciò che BombVault salva sta in un unico elenco sotto **Backup**: container, VM, set di cartelle, la chiavetta flash, dataset ZFS e l'auto-backup. Ognuno è un **elemento** con una pagina propria. Questa pagina illustra l'elenco, la pagina dell'elemento con le sue tre schede e le impostazioni che un elemento può avere.

![L'elenco Backup con il riquadro Container selezionato: ogni riga mostra le sue ultime esecuzioni come una striscia e un unico stato.](assets/screenshots/backups/list-dark.png#only-dark)
![L'elenco Backup con il riquadro Container selezionato: ogni riga mostra le sue ultime esecuzioni come una striscia e un unico stato.](assets/screenshots/backups/list-light.png#only-light)

## Cosa salva ogni sezione {#what-is-saved}

| Sezione | Cosa viene salvato |
|---|---|
| **Container** | La cartella appdata più la definizione del container: immagine, variabili, porte, label e volumi. La scheda **Cartelle da salvare** nella scheda **Impostazioni** del container spunta con precisione quali cartelle copre il backup, con un conteggio in tempo reale dei percorsi e un elenco di ciò che hai lasciato fuori. I dati che un container tiene fuori da Host Data, per esempio in un ixVolume di TrueNAS, non vengono salvati, e l'esecuzione nomina le cartelle che ha lasciato fuori. |
| **VM** | Le immagini disco, la definizione XML e la NVRAM UEFI, con uno spegnimento ordinato o uno snapshot a caldo, via SSH. Vedi [Backup delle VM](vm-backup-ssh-setup.md). |
| **Flash** | Solo Unraid. L'intera chiavetta USB (`/boot`): sistema operativo, licenza, configurazione dell'array, condivisioni, rete e configurazione dei plugin. |
| **Auto-backup** | Il `/config` di BombVault stesso: il suo database delle impostazioni, le credenziali delle destinazioni e la coppia di chiavi SSH di libvirt, copiati con `VACUUM INTO` di SQLite così il database non viene mai catturato nel mezzo di una scrittura. |
| **Cartelle** | Qualsiasi cartella del server come set di cartelle, per esempio una condivisione, i tuoi documenti o una libreria di foto, con pattern di esclusione opzionali. I set di cartelle hanno pianificazioni, conservazione, copie off-site, controlli e prove di ripristino come ogni altra cosa. |
| **Dataset ZFS** | Un dataset con tutti i dataset sottostanti, letto da un unico snapshot ZFS così che tutti provengano dallo stesso istante, e salvato con restic come una cartella. Un elemento ZFS può anche tenere una replica identica a livello di blocchi su un secondo server ZFS. Vedi [Dataset ZFS](zfs-datasets.md). |

Ogni backup passa da restic, quindi è incrementale e deduplicato, e nemmeno i grandi dischi di VM gonfiano il repository. BombVault non salva mai il proprio container, e quindi non lo ferma mai. Un'esecuzione può essere fermata senza rischi, perché restic scrive lo snapshot per ultimo, così un'esecuzione interrotta lascia dati senza riferimenti e nessuno snapshot. Un lock rimasto dopo un riavvio nel mezzo di un'esecuzione viene rimosso e l'esecuzione riprovata una volta, e solo dopo che il lock è rimasto dieci minuti senza essere rinnovato.

## L'elenco {#list}

I riquadri sopra l'elenco scelgono un tipo, come **Container** o **ZFS**, e ogni riquadro conta i suoi elementi. Un riquadro seleziona, non nasconde il resto della pagina.

Ogni riga mostra le ultime quattordici esecuzioni dell'elemento come piccoli quadrati e un unico stato, come **Protetto**, **Non protetto**, **Rinviato** o **Replica in corso**. Tutto il resto lo trovi nel fumetto informativo accanto al suo nome, e finché il puntatore sta sulla riga quel testo compare nella riga stessa. Un segno accanto al nome di un container dice che dall'ultimo backup è stato ricreato con un'altra immagine, altre porte, variabili o volumi. Sparisce con il backup successivo.

Tre pulsanti sopra l'elenco lo restringono:

- **Filtri**: per **Calendario** (**Tutti**, **Pianificato**, **In pausa**), per **Backup** (**Con backup**, **Mai sottoposto a backup**) e, per **Container e VM**, **Installati** o **Non installato**.
- **Ordina**: per nome, stato, IP, ultimo backup, dimensione o calendario. Ogni campo tranne l'IP si ordina in entrambi i sensi, come **Dimensione, prima i più grandi** e **Dimensione, prima i più piccoli**.
- **Cerca**: per nome, immagine o IP.

Gli elementi che non sono più installati ma hanno ancora backup stanno a parte sotto **Non installato (solo backup)**. Ripristinali, oppure elimina i loro backup per liberare spazio.

L'azione principale della pagina galleggia in basso a destra: **Esegui il backup di tutto ora** avvia un'esecuzione su ogni sezione attivata. **Altro** contiene il resto:

- **Seleziona** avvia una selezione, vedi sotto.
- **Scopri backup** legge ciò che contengono le destinazioni e ne ricostruisce gli elementi, per esempio dopo un'installazione nuova. Le copie off-site delle sezioni ricostruite restano allora in pausa finché non confermi il loro valore predefinito, vedi [Dopo una ricostruzione](offsite-recovery.md#after-a-rebuild).
- **Connessione ZFS a questo server** apre la prova SSH della sezione ZFS, vedi [Dataset ZFS](zfs-datasets.md#requirements).
- **Replica tutto ora**, con il riquadro ZFS selezionato, invia la replica di ogni elemento ZFS.

![Lo stesso elenco su un telefono.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Lo stesso elenco su un telefono.](assets/screenshots/backups/list-phone-light.png#only-light)

## Selezionare più elementi {#select}

Scegli le righe come scegli i file in un file manager: Ctrl+clic, Maiusc+clic, Ctrl+A, oppure una pressione lunga su un telefono. Una riga scelta mostra un segno di spunta al posto della sua icona, e una barra sopra l'elenco li conta e offre **Backup selezione**, **Sospendi pianificazione** e **Riprendi pianificazione**, **Seleziona tutto** e **Annulla selezione**. Ogni riga ha anche un menu con il clic destro con le sue azioni.

Una selezione viene salvata sul server, un elemento dopo l'altro, quindi continua anche quando chiudi la scheda. Per ripristinare più elementi insieme usa la [pagina Ripristino](restore.md#several).

![Due container selezionati, con la barra sopra l'elenco.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Due container selezionati, con la barra sopra l'elenco.](assets/screenshots/backups/bulk-light.png#only-light)

## Aggiungere una cartella o un dataset ZFS {#add}

Container, VM e la chiavetta flash compaiono da soli quando la loro sezione è attivata in **Impostazioni, Generale**. Cartelle e dataset ZFS li scegli tu, perché BombVault non può indovinare quali intendi. **Aggiungi** apre **Cos'altro deve salvare BombVault?**:

- **Una cartella** elenca le condivisioni che BombVault ha trovato e accetta qualsiasi altra cartella sotto Host Data. Dai un nome al set di cartelle, per esempio `Documents`. Il nome è fisso dopo il primo backup, perché gli snapshot sono salvati sotto di esso.
- **Un dataset ZFS** elenca i dataset di questo server. Scegli quello in cima a ciò che vuoi salvare, vedi [Elementi e dataset figli](zfs-datasets.md#items-and-children).
- **Configurazione di sistema** compare su un host senza Unraid il cui Host Data raggiunge `/etc`. È un set di cartelle `/etc` di partenza che controlli prima di salvare.

## Lasciare fuori un elemento {#dont-back-up}

Un elemento che lasci fuori di proposito, come un container di prova, può essere segnato **Non fare il backup**. Smette allora di contare come non protetto, e **Non escludere più** lo riporta dentro. **Sospendi pianificazione** è la via più morbida: l'elemento esce dalla pianificazione e dal backup completo, porta un badge grigio **Pianificazione sospesa**, e **Esegui backup ora** funziona ancora. Imposta lo stesso interruttore di **Nel calendario** nella scheda Impostazioni dell'elemento.

## La pagina di un elemento {#entry-page}

Un clic su una riga apre la pagina dell'elemento. L'intestazione nomina l'elemento e il suo stato in una riga, come **Ultimo backup 3 ore fa** o **Rinviato, prossimo tentativo alle 03:15**, e contiene le sue azioni: **Esegui backup ora**, **Ripristina**, e sotto **Altro** quelle rare, come **Esporta il backup più recente**, **Esporta container** o **Esporta VM**, **Collega a un elemento precedente**, **Elimina tutti i backup** e **Rimuovi elemento**. Un pulsante indietro a sinistra dell'icona porta all'elenco. Mentre un backup è in corso, **Annulla backup** sta accanto al suo avanzamento, e un'esecuzione annullata viene registrata come annullata, non come fallita.

La pagina ha tre schede: **Backup**, **Impostazioni** e **Verifica**.

### Scheda Backup {#backups-tab}

Questa scheda elenca gli snapshot dell'elemento, il più recente per primo, con una barra per giorno. Un backup che si trova in più posti compare una volta, contrassegnato con ogni destinazione che lo contiene. Un clic su uno snapshot lo ripristina, e un clic su un punto dentro uno snapshot lo ripristina esattamente da lì.

- **Destinazioni** nomina ogni posto con backup di questo elemento e quanti. I posti off-site vengono letti solo quando premi **Verifica**, perché elencare un archivio cloud richiede un po'. Un posto su cui l'elemento non salva più è indicato come **Solo backup precedenti** e si può ancora ripristinare da lì.
- Ogni snapshot ha un menu: **Aggiungi tag** o **Cambia tag**, **Confronta con un altro backup**, che elenca i file arrivati, spariti o cambiati, **Esporta** come tar, zip o un repository restic a sé, ed eliminazione in un solo posto.
- I **Dump del database** di un container stanno in un elenco a parte, con download, salvataggio in una cartella e importazione.

Quanti snapshot restano lo decide la destinazione, vedi [Per quanto tempo conservare](offsite-recovery.md#keep).

![La scheda Backup di un container: una barra per giorno e le destinazioni che contengono ogni snapshot.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![La scheda Backup di un container: una barra per giorno e le destinazioni che contengono ogni snapshot.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Scheda Impostazioni {#settings-tab}

Ogni elemento comincia con **Calendario e destinazione**:

- **Nel calendario** inserisce o toglie l'elemento dalle esecuzioni pianificate.
- **Piano proprio** dà a questo singolo elemento un piano suo, vedi [Un piano proprio per un elemento](schedules.md#own-plan). Senza, gira con il piano della sua sezione o nel backup completo.
- Una fila di pulsanti, **Locale** e uno per ogni destinazione off-site, dice dove vanno i backup. **Segue il valore predefinito** significa che l'elemento va dove va la sua sezione, e un clic su di esso apre **Destinazioni per sezione**. Vedi [Destinazioni per elemento](offsite-recovery.md#placement).
- **Repository** nomina dove vengono scritti i backup (**Salvato su**). Quel posto è fisso dal primo backup in poi, le copie possono cambiare in qualsiasi momento.

Le schede sotto dipendono dal tipo di elemento:

| Tipo | Schede |
|---|---|
| Container | **Cartelle da salvare** (quali cartelle mappate vengono salvate, con **Salta le cartelle di cache (CACHEDIR.TAG)**), **Pattern di esclusione**, **Arresta altri container**, **Comandi prima e dopo**, **Database**, **Quando l'app è occupata**, **Aggiorna dopo un backup riuscito**, **Snapshot invece dello stop**, **Nomi precedenti** |
| VM | **VM** con **Metodo** (**Ordinato (spegnimento)** o **Snapshot a caldo**) e **Solo blocchi modificati**, vedi la [guida al backup delle VM](vm-backup-ssh-setup.md#per-vm-method) |
| Set di cartelle | **Cartella** e **Pattern di esclusione** |
| Chiavetta flash | **Esportazione zip del flash**, che scrive ogni backup della flash come semplice `.zip` in una cartella a tua scelta |
| Dataset ZFS | i dataset figli, i container da fermare e i comandi attorno allo snapshot, **Replica** e **Prova l'accesso agli snapshot**, vedi [Dataset ZFS](zfs-datasets.md) |

Le impostazioni si salvano da sole mentre le cambi.

![La scheda Impostazioni di un container, con Piano proprio nella prima scheda.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![La scheda Impostazioni di un container, con Piano proprio nella prima scheda.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Scheda Verifica {#verify-tab}

- **Controlli** mostra la verifica di ripristino che segue il primo backup. **Verifica ripristino** la esegue di nuovo sul backup più recente: BombVault ripristina un campione di al massimo 100 file e 256 MiB in una cartella temporanea e fa rileggere a restic ogni file confrontandolo con i suoi hash.
- **Test di avvio**, su un container, ripristina il backup più recente in una copia isolata e la avvia: un nome che inizia con `bombvault-test-`, una rete Docker interna propria senza porte pubblicate e senza vie verso la LAN, 1 CPU e 2 GiB di memoria. Riesce se la copia risulta sana, risponde sulla sua prima porta esposta oppure, in mancanza di entrambe, resta in esecuzione. L'originale non viene mai toccato, e la copia, la sua rete e i suoi dati vengono rimossi alla fine, anche dopo un riavvio nel mezzo di un test. I container sulla rete host, quelli privilegiati, quelli con dispositivi e quelli che hanno bisogno di un altro container risultano non testabili. **Test di avvio** in **Impostazioni, Verifica, Controlli automatici** prova un container per esecuzione, quello provato da più tempo per primo.
- **Anomalie** imposta la sensibilità e il livello di notifica propri di questo elemento e mostra i suoi riscontri aperti.
- **Dimensione per cartella** mostra cosa occupa spazio nel backup più recente e quanto di esso ha portato l'ultimo backup, un livello alla volta.
- **Esecuzioni recenti** elenca le ultime esecuzioni con inizio, fine e durata. Un clic mostra i dettagli.

![La scheda Verifica di un container con Dimensione per cartella aperta.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![La scheda Verifica di un container con Dimensione per cartella aperta.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Pattern di esclusione {#excludes}

La scheda **Pattern di esclusione** accetta un pattern per riga. Scrivi i percorsi come li vedi dentro il container, come `/config/Library/Application Support/Plex Media Server/Cache`, oppure un nome semplice come `.git`, che corrisponde a qualsiasi profondità. Un'anteprima sotto il campo mostra a cosa corrisponde ogni riga, e avvisa quando una riga non corrisponde a nulla o escluderebbe tutto. Una riga con `/` o `*`, o una che nomina un'intera cartella salvata, viene ignorata, così un'esclusione non può mai svuotare un backup. Per lasciare fuori un'intera cartella, deselezionala invece in **Cartelle da salvare**.

Per le immagini note come Plex, Jellyfin, Sonarr, Radarr, Immich e Nextcloud la scheda propone le cartelle che l'app riempie di nuovo da sola, come cache, log e miniature. Nulla viene escluso finché non premi **Escludi selezionati**.

La stessa scheda porta avvisi che la scansione di una cartella non può vedere. Immich, per esempio, tiene album, volti e date in un database PostgreSQL in un container separato, quindi un backup dei file del solo container Immich ripristina le immagini senza nulla di tutto questo, e il ripristino sembra riuscito. Salva anche il container del database.

## Dump dei database {#database-dumps}

I container PostgreSQL, MySQL e MariaDB che BombVault riconosce vengono scaricati in un dump prima di ogni backup, dal server in esecuzione, in un punto di ripristino a sé accanto ai file. Questo copre le immagini ufficiali, PostGIS, TimescaleDB, pgvector, pgautoupgrade, le immagini di database di Immich, le immagini MariaDB di linuxserver, yobasystems e jc21 e mysql-server di Oracle. Un container che somiglia soltanto a un database ottiene la stessa opzione, disattivata finché non la scegli.

Il dump scorre direttamente nel repository e non viene mai scritto su un disco. Le credenziali vengono dalle variabili del container stesso, compresi i segreti `*_FILE`, e non lo lasciano mai. La scheda **Database** nella scheda Impostazioni del container dice se la cartella dei dati viene salvata da ferma, copiata mentre gira o non salvata, e attiva o disattiva il dump per questo container. L'etichetta `bombvault.dbdump=false` fa lo stesso e prevale sulla scheda. **Dump automatici dei database** in **Impostazioni, Generale** lo imposta per tutti.

Un dump fallito non fa fallire il backup. Compare come esecuzione fallita a sé con il motivo e invia una notifica. I dump non vengono mai ricaricati da soli: scaricane uno, semplice o compresso, salvalo in una cartella oppure importalo in un database appena avviato con un clic nella scheda **Backup**. Per riavere un dump senza BombVault vedi [Riavere un dump del database](offsite-recovery.md#database-dumps). Quando uno fallisce vedi [Un dump del database è fallito](troubleshooting.md#a-database-dump-failed).

## Comandi e altri container {#commands}

- **Comandi prima e dopo** girano dentro il container con `sh -c`, per esempio per scrivere una cache su disco. Un comando prima del backup che fallisce lo interrompe. Un comando prima del backup che esce con il codice 75 rinvia invece un backup pianificato, vedi sotto.
- **Arresta altri container** nomina i container da fermare mentre questo viene salvato, come il suo database. Poi ripartono nel loro ordine `depends_on` di Compose, e di default ciascuno deve risultare sano prima che parta il successivo. L'attesa e il suo timeout si trovano sotto **Riavvio dopo il backup** nella scheda **Ordine** in **Impostazioni, Calendario**.

## Quando l'app è occupata {#app-busy}

Un backup ferma il suo container, quindi uno pianificato può aspettare mentre qualcuno usa l'app e riprovare da solo. La scheda **Quando l'app è occupata** nella scheda Impostazioni di un container lo configura:

- **Controllo attività** chiede a Plex, Jellyfin o Emby prima di un backup pianificato se qualcuno sta guardando. Plex viene interrogato con il token del suo `Preferences.xml`, quindi non c'è nulla da inserire. Jellyfin ed Emby richiedono una **Chiave API**, creata nella dashboard del server sotto API keys.
- **Ignora le sessioni in pausa dopo (min)** fa smettere di contare un film messo in pausa per la notte.
- **Esegui il backup quando il server non risponde** decide cosa succede quando non si può interrogare il server: attivo, il backup parte e il registro lo dice. Disattivo, aspetta come se qualcuno stesse guardando.
- **Attendi mentre il container invia dati** serve per le app senza un controllo simile, come un server di giochi o un downloader. Il backup aspetta finché il container ha inviato più di **Occupato da (KiB/s)** in una qualsiasi lettura degli ultimi **Tranquillo per (min)** minuti.
- **Riprova ogni (min)**, **Prova solo in una fascia oraria** con **Prova tra** e **Attendi al massimo (ore)** stabiliscono come procedono i tentativi.
- **Alla fine dell'attesa** rinuncia, il che mostra il backup sotto **Richiede attenzione** con **Riprova**, oppure fa **Esegui comunque il backup**.

Un backup in attesa viene registrato come **Rinviato**, in ambra, e il resto dell'esecuzione prosegue. La panoramica, l'elenco e l'elemento lo mostrano con il prossimo tentativo, e l'avviso di ritardo e lo stato di protezione lo aspettano fino alla sua scadenza. Ricevi una notifica quando viene rinviato e una quando finisce, non una per ogni tentativo. Un tentativo aspetta mentre gira un altro backup, e un'attesa continua dopo un riavvio. Un backup che avvii tu parte sempre subito, sia con **Esegui backup ora**, sia con **Riprova**, con una selezione, con l'API o con MCP, e uno che passa chiude un'attesa in sospeso.

La scheda Impostazioni del container ha anche un modo più vecchio e semplice: **Attendi che l'app sia inattiva** con **Attendi al massimo (ore)**. Il backup parte non appena l'app è inattiva, cosa che definisce la scheda **Inattività prima del backup** in **Impostazioni, Calendario**. I membri di uno stack Compose in programma nella stessa esecuzione aspettano insieme, e il backup in attesa non tiene alcun lock, così gli altri container proseguono.

![La scheda Quando l'app è occupata di un container Plex con il controllo attività configurato.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![La scheda Quando l'app è occupata di un container Plex con il controllo attività configurato.](assets/screenshots/backups/app-busy-light.png#only-light)

### Esempio: Plex aspetta mentre qualcuno guarda {#example-plex}

Il backup completo gira ogni notte alle 03:00. Nessuno dovrebbe vedere un film interrompersi perché Plex si è fermato per il suo backup, ma il backup deve comunque avvenire prima del mattino.

1. Apri **Backup**, poi l'elemento `plex`, poi la sua scheda **Impostazioni**.
2. In **Quando l'app è occupata** imposta **Controllo attività** su Plex. Non c'è nulla da digitare.
3. Imposta **Ignora le sessioni in pausa dopo (min)** su `30`, così un film messo in pausa a mezzanotte smette di contare a mezzanotte e mezza.
4. Lascia attivo **Esegui il backup quando il server non risponde**.
5. Imposta **Riprova ogni (min)** su `15`.
6. Attiva **Prova solo in una fascia oraria** e imposta **Prova tra** su `22:00` e `06:00`.
7. Imposta **Attendi al massimo (ore)** su `3` e **Alla fine dell'attesa** su **Esegui comunque il backup**.

Cosa succede in una notte in cui qualcuno sta ancora guardando alle 03:00: BombVault interroga Plex, trova una sessione in riproduzione e registra il backup come **Rinviato** con il prossimo tentativo alle 03:15. Gli altri container dell'esecuzione vengono salvati come al solito. Quando il film finisce alle 04:10, il tentativo delle 04:15 trova Plex inattivo e lo salva, e l'attesa finisce con una notifica. Se il film fosse ancora in corso alle 06:00, l'attesa arriverebbe alla sua fine e Plex verrebbe salvato comunque, così i suoi backup non restano mai indietro di più di tre ore.

Per un tuo controllo, fai uscire il **Comando pre-backup** con il codice 75 mentre l'app è occupata, per esempio `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Altri sintomi e rimedi sono in [Risoluzione dei problemi](troubleshooting.md#deferred-backups).

## Esportazioni semplici {#exports}

restic resta il motore, ma un'esportazione ti dà una copia che non richiede né BombVault né restic.

- **Esporta container** ed **Esporta VM** sotto **Altro** nella pagina di un elemento scrivono una copia com'è in questo momento, accanto al repository: le cartelle da salvare del container più il suo template Unraid, oppure le immagini disco della VM più la sua definizione, ripristinabile con `virsh define`.
- **Esporta** su uno snapshot scrive un backup salvato nella cartella di esportazione del server, come tar, zip o un repository restic a sé, e può prendere uno snapshot o tutti.

Le esportazioni sono file semplici di default. **Cifra le esportazioni con age** in **Impostazioni, Sistema** le sigilla, ZIP della flash compreso, per i destinatari che elenchi, ciascuno una chiave pubblica age o una chiave pubblica SSH. Le decifri più tardi fuori dal server con la chiave privata corrispondente. Con la cifratura attiva e nessun destinatario valido, un'esportazione fallisce con un errore chiaro invece di scrivere mai file in chiaro.

## Cosa ti dice un'esecuzione {#run-notes}

- **Tempi.** Ogni esecuzione riporta inizio, fine e durata, e ogni elemento ha le sue **Esecuzioni recenti**.
- **Perché un backup è stato lento.** Mentre un backup gira, BombVault osserva quanto sono occupati la CPU, i dischi e la rete. Quando un backup dura molto più del solito e una cosa era chiaramente al limite, l'esecuzione lo dice, per esempio "Il disco di destinazione disk1 era occupato al 98%". Altrimenti non dice nulla.
- **Avanzamento di controlli e potature.** Il registro attività e la scheda **Controlla i backup** mostrano fin dove ha contato restic, come *12 di 47 pack*, con il tempo rimasto in quel passo quando c'è abbastanza per stimarlo.
- **File che non ha potuto leggere.** Un backup che è andato a buon fine ma ha lasciato fuori qualcosa colora di arancione la sua riga e nomina ciò che è stato lasciato fuori.
