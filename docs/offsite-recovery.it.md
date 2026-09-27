# Off-site e ripristino

I backup locali ti proteggono da un container perso o da un aggiornamento andato male. La replica off-site e un kit di ripristino testato ti proteggono dall'intera macchina, dal ransomware o da un incendio. Questa pagina copre la replica off-site, il rendere quella copia a prova di manomissione, il dimostrare di poter ripristinare e il recuperare quando BombVault stesso non c'è più.

## Replica off-site

Mantieni il backup locale veloce e copialo in uno o più altri luoghi. I luoghi in cui viene copiato un dominio si scelgono nella scheda **Domini** in **Impostazioni, Archiviazione**, un chip per luogo (vedi [Luoghi di archiviazione](storage-places.md#domains)). BombVault vi copia i nuovi snapshot con `restic copy` su base best-effort, così una copia non riuscita non fa mai fallire il backup locale. Il luogo in cui è salvato un dominio non deve per forza essere locale; vedi [Un dominio salvato in un luogo remoto](#remote-primary-repositories).

- **Più luoghi di copia per dominio.** Un dominio può essere copiato in più luoghi contemporaneamente, per esempio un rest-server a casa di un amico e un bucket B2. Conservazione, classe di archiviazione, append-only, limiti e budget di crescita appartengono al luogo, così ogni copia segue le regole del luogo in cui arriva.
- **Calendario di copia per dominio** (modificato insieme a ogni altro calendario su Impostazioni, Calendari): lascialo vuoto per copiare dopo ogni backup locale, oppure imposta una cadenza (per esempio `weekly Sun 03:00`) per copiare meno spesso di quanto esegui il backup. **Copia ora** sulla riga del dominio lo esegue su richiesta.
- **Conservazione per luogo.** Ogni luogo mantiene le proprie regole, così un luogo off-site può tenere le copie più a lungo come archivio. Un luogo con tutte le regole a zero non taglia mai.
- **I limiti di banda** per luogo limitano la velocità di upload e download di restic così la copia non satura la tua WAN.
- Un **indicatore di replica** mostra quale dominio sta copiando mentre è in corso (sulla sua pagina e sulla Dashboard). È un indicatore attivo, non una barra di percentuale, perché `restic copy` non espone alcun progresso leggibile da una macchina.

!!! note "Ripristina da qualsiasi luogo"
    Ogni container, VM, set di file, la flash e la configurazione dell'app elencano i propri backup come un'unica cronologia su tutti i luoghi in cui si trova un backup. Un backup copiato su B2 compare una sola volta, contrassegnato con ogni luogo che lo contiene. Un ripristino usa il primo luogo che riesce a raggiungere, a partire dal repository su cui l'elemento viene scritto, e puoi scegliere un altro luogo per ogni riga. I luoghi off-site vengono letti solo quando li apri. L'eliminazione in un luogo controlla prima gli altri e dice se era l'ultima copia.

## Collocazione per elemento {#placement}

Ogni scheda di container, VM e set di file ha una riga **Collocazione** con tre segmenti:

- **Locale** scrive l'elemento sul repository mostrato sotto **Salvato su** e non lo copia da nessuna parte. Usalo per dati che hanno già una seconda copia, per esempio una condivisione che vive su un NAS.
- **Locale + off-site** lo scrive anche lì e lo copia sulle destinazioni spuntate sotto **Copia su**, un chip per ogni destinazione off-site del dominio. Togli la spunta a un chip e quella destinazione non riceve più nulla di nuovo da questo elemento.
- **Solo off-site** scrive l'elemento direttamente nel luogo sotto **Invia a**, qualsiasi luogo diverso dal luogo principale del dominio. Se il dominio viene già copiato in quel luogo, l'elemento riceve un repository diretto accanto alle copie; altrimenti BombVault crea lì un repository per il dominio.

La posizione è fissa dal primo backup dell'elemento in poi, perché BombVault non sposta mai i backup tra repository. Le copie possono cambiare in qualsiasi momento. Una destinazione che non riceve più un elemento mantiene le copie che ha e le pota secondo la propria conservazione alla prossima esecuzione off-site del dominio; **Elimina in B2** sulla scheda le rimuove subito. Quando alcune di quelle copie non esistono da nessun'altra parte, la conferma le elenca per data e chiede il nome dell'elemento. Dalle destinazioni append-only non si può eliminare.

Sotto la riga la scheda indica dove va l'elemento e cosa c'è realmente: quante sedi lo custodiscono, quando ogni destinazione è stata vista l'ultima volta, e se il 3-2-1 è rispettato. Una sede è il server con i dati originali e ogni luogo in un altro sito (vedi [Fuori sede](#off-the-premises-mark)). BombVault controlla le copie e le sedi; non controlla la parte «due supporti» del 3-2-1.

### Valori predefiniti per dominio

La scheda **Domini** in Impostazioni, Archiviazione ha una riga per dominio. **Copiato su** si applica subito a ogni elemento senza una scelta propria, e alle cartelle di progetto degli stack Compose. Una volta che un dominio ha dei backup, **Salvato su** si applica a un nuovo elemento al suo primo backup, e cambiarlo non sposta nessun backup. Prima di salvare, la riga elenca ogni luogo che guadagna o perde elementi e quanti snapshot significa, e la domanda contiene l'interruttore **Applica agli elementi senza backup**, che riporta anche sul nuovo valore predefinito ogni elemento ancora senza backup. **Eccezioni** elenca gli elementi con una scelta propria.

Spuntare un nuovo luogo in **Copiato su** gli fa ricevere ogni elemento non impostato su Locale. La conferma indica quanti elementi e, dove noto, quanta cronologia significa.

### Repository diretti

Scegliere sotto Solo off-site un luogo in cui il dominio viene già copiato chiede conferma una volta, poi crea un repository diretto accanto alle copie, per esempio `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, e vi punta l'elemento. Per una destinazione di copia senza luogo, la scelta apre una finestra con un indirizzo suggerito e un test di connessione che non crea nulla, e **Crea e usa** crea il repository. Un repository diretto prende la chiave, la classe di archiviazione, i limiti, l'impostazione append-only e la conservazione del luogo, e cambia insieme a essi. Quando una nuova chiave del luogo non riesce ad aprirlo, il repository diretto mantiene la chiave che ha e il salvataggio lo segnala. I suoi snapshot portano il tag `bv:direct`, e ogni altro passaggio di conservazione li risparmia, così un repository diretto che ha perso il collegamento con il suo luogo non invecchia mai secondo le regole locali. Una chiave B2 limitata a una cartella deve coprire l'indirizzo del luogo, non solo la cartella del dominio, altrimenti la cartella accanto resta irraggiungibile.

### Fuori sede {#off-the-premises-mark}

Una copia conta come sede a sé solo quando il suo luogo si trova in un altro sito. Un luogo cloud conta sempre e una cartella su questo Unraid mai; per un NAS, un rest-server o un server SFTP, rispondi a **Dove si trova il dispositivo?** nei dettagli del luogo con **Qui in casa** o **In un altro sito**. La risposta conta solo le sedi e il 3-2-1 sulle schede e sulla Dashboard. Non cambia nessuna copia.

### Dopo una ricostruzione

Le scelte di copia vivono nelle impostazioni di BombVault stesso. Dopo una ricostruzione tramite Scopri senza un `/config` ripristinato sono perse, e copiare tutto rimanderebbe su B2 gli elementi che avevi escluso. La replica off-site di ogni dominio ricostruito quindi si mette in pausa. La Dashboard lo mostra in ambra, e la riga del dominio nella scheda Domini offre **Conferma valore predefinito** con un'anteprima di cosa copia la prossima esecuzione e i nomi nei backup che non hanno una voce, che puoi escludere lì. Solo la conferma termina la pausa; importare un file di impostazioni riporta regole e valori predefiniti ma non la termina.

## Un dominio salvato in un luogo remoto {#remote-primary-repositories}

Un dominio non deve per forza essere salvato in locale. Finché la sua posizione di backup non contiene backup, scegli un luogo remoto in **Salvato su** nella scheda Domini e il dominio esegue il backup direttamente lì, senza copia locale e senza passo di copia. Il repository remoto è allora l'unica copia, a meno che il dominio non venga copiato anche in un altro luogo. Ogni luogo remoto ha le stesse salvaguardie:

- **Un test di connessione** prima che venga scritto qualcosa.
- **Limiti di banda** per il backup stesso, le stesse opzioni `--limit-upload` e `--limit-download` usate da una copia.
- **Protezione append-only**, verificata con lo stesso test di manomissione attivo. Con essa attiva, BombVault non pota mai il repository, perché le credenziali su questa macchina non devono poter cancellare l'unica copia del backup.
- **Un budget di crescita**, ricavato dallo stesso andamento della dimensione tracciato dalla scheda Archiviazione.

Un dominio salvato in un luogo remoto è la sorgente delle sue copie come uno locale; vedi [Copie tra luoghi con credenziali diverse](storage-places.md#different-credentials).

!!! note "Le credenziali appartengono al luogo"
    Un luogo remoto conserva le proprie credenziali. Un luogo configurato con le credenziali cloud condivise continua a usarle finché il suo accesso non viene cambiato nei suoi dettagli.

## Off-site immutabile (append-only)

Contrassegna un repo off-site come append-only così ransomware, o un host compromesso, non possano eliminare o riscrivere i tuoi backup. L'altro lato (un `restic/rest-server` in esecuzione in modalità `--append-only`) lo **impone**. BombVault lo **verifica** soltanto e non mostra mai verde sulla sola affermazione di una configurazione.

La finestra **Aggiungi luogo** contiene una ricetta pronta da incollare per un rest-server in modalità append-only, con un utente per questo BombVault. In un luogo rest-server con **Append-only** attivo, **Verifica append-only** nei dettagli del luogo esegue il tamper test su ogni percorso di dominio, copia attiva e repository del luogo e dà un'unica risposta per il luogo, così l'off-site append-only è raggiungibile senza modificare a mano le configurazioni.

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

In un luogo, **Verifica append-only** sonda ogni percorso di dominio, copia attiva e repository presenti con le loro credenziali e riunisce i verdetti in un'unica risposta: basta un solo repository che accetta un'eliminazione perché l'intero luogo risulti *eliminazione accettata*.

## Esercitazioni DR

BombVault offre due livelli di prova che i tuoi backup siano effettivamente ripristinabili, non solo presenti.

- **Esercitazioni di verifica del ripristino (locali).** BombVault esegue periodicamente `restic check --read-data-subset` (limitato, mai un ripristino completo che riempie il disco) e mostra un badge *ultima ripristinabilità verificata* per dominio. La cadenza risiede su Impostazioni, Calendari; il badge su Impostazioni, Integrità.
- **Esercitazioni DR (off-site).** BombVault ripristina una destinazione reale dal repo off-site in una sandbox usa e getta, la verifica file per file e byte per byte, poi ripulisce. Questo dimostra che puoi recuperare da off-site, non solo che il repo risponde. Vengono esercitati solo i luoghi in un altro sito, perché una copia nella stessa casa non dimostra nulla sulla perdita della casa. Un dominio copiato su più di essi viene esercitato su uno per ogni esecuzione pianificata, a turno, e la Dashboard indica il luogo dell'ultima esercitazione.

La **scorecard della protezione dal ransomware** sulla Dashboard riassume tutto questo in una postura verde / ambra / rossa per dominio, con una checklist con marca temporale (off-site configurato, append-only verificato, replica aggiornata, esercitazione di ripristino superata, cifratura attiva, strategia di pota impostata). Ogni riga rossa collega direttamente alla soluzione, e la scheda diventa verde solo su fatti verificati.

## Dashboard ricevente (il lato ricevente)

![Il lato ricevente, osservato in sola lettura, con un controllo di integrità eseguito su questa macchina.](assets/screenshots/receiver.png)

*Il lato ricevente, osservato in sola lettura, con un controllo di integrità eseguito su questa macchina.*

Tutto quanto sopra è il lato *mittente*. Sulla macchina che **riceve** copie off-site immutabili da un altro BombVault, la dashboard Ricevente ti offre un monitoraggio indipendente e in sola lettura di quei repository sull'hardware ricevente, così un fallimento silenzioso all'altra estremità non passa inosservato.

Attiva l'interruttore **Ricevente** in Impostazioni per rivelare una scheda **Ricevente**. È disattivato di default; abilitalo solo su una macchina che riceve effettivamente backup off-site immutabili. Poi registra un repository ricevuto (in sola lettura, aperto con la chiave dell'istanza mittente) per ottenere:

- **Un inventario di snapshot raggruppato per sorgente**, così puoi vedere esattamente quali container, VM e set di file sono arrivati.
- **Ultimo ricevuto** per sorgente, così sai quanto è fresco ciascuno.
- **Un `restic check` indipendente** eseguito sull'hardware ricevente, così l'integrità viene verificata dove i dati effettivamente risiedono, non solo sul mittente.
- **Un dead-man's switch:** un avviso quando una sorgente smette di inviare entro una finestra che imposti.
- **Avvisi di integrità:** un avviso quando un controllo sul lato ricevente fallisce.

Il Ricevente è rigorosamente in sola lettura. Non scrive mai nel repository ricevuto, così non può mai rompere la garanzia append-only su cui il mittente fa affidamento.

## Esempio completo: due macchine Unraid, dall'inizio alla fine

Sopra sono descritti i pezzi. Questa è un'installazione completa con valori reali, perché i pezzi si montano meglio dopo averli visti montati una volta.

Due macchine: **TOWER** esegue i container e invia i backup, **VAULT** li riceve e impone l'immutabilità. Sostituisci con i tuoi nomi, indirizzi e percorsi di condivisione.

**1. Su VAULT, avvia il server append-only.** In BombVault su TOWER apri *Impostazioni → Archiviazione*, clicca **Aggiungi luogo**, scegli **rest-server** e clicca **Mostra ricetta**. Copia il blocco **Template Unraid**, salvalo su VAULT come `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, poi *Docker → Add Container* e scegli **rest-server** dall'elenco dei modelli. Prima di avviarlo, scrivi la riga `htpasswd` mostrata in `/mnt/user/appdata/rest-server/.htpasswd` su VAULT. La password viene mostrata una sola volta e non viene mai conservata; la ricetta l'ha già inserita nel modulo su TOWER insieme all'utente, quindi lascia aperta quella finestra. La riga `htpasswd` porta la stessa password, già trasformata in hash bcrypt per te, quindi non devi calcolare nessun hash tu.

    Lascia `--append-only` nel campo OPTIONS. Senza, VAULT torna a essere solo una normale condivisione.

**2. Su TOWER, aggiungi il luogo.** Inserisci l'indirizzo di VAULT, `http://VAULT:8000`, accanto all'utente e alla password compilati dalla ricetta, poi clicca **Prova connessione**. BombVault costruisce l'indirizzo da questi dati:

    rest:http://VAULT:8000/tower

Il primo segmento del percorso è l'utente htpasswd, qui `tower`, e ogni dominio riceve la sua cartella al di sotto, per esempio `rest:http://VAULT:8000/tower/container`. Rispondi a **Dove si trova il dispositivo?** con **In un altro sito**, clicca **Aggiungi** e spunta il luogo in **Copiato su** per i domini che devono andare lì.

**3. Su TOWER, attiva Append-only** in **Protezione** nei dettagli del luogo, poi clicca **Verifica append-only**. Il test sonda ogni percorso di dominio, copia e repository del luogo e dà un'unica risposta per il luogo, che deve essere *eliminazione rifiutata*. Cosa significano le risposte:

| Risultato | Cosa è successo |
| --- | --- |
| **eliminazione rifiutata** | VAULT ha rifiutato la cancellazione. È l'unico stato che passa. |
| **eliminazione accettata** | VAULT ha accettato una cancellazione. Manca `--append-only` oppure è stato rimosso. |
| un messaggio al posto di un risultato | Non è stato possibile eseguire il test. Di solito l'indirizzo non è quello che usa restic, oppure le credenziali sono cambiate. Non viene registrato nulla e non scatta alcun avviso. |

**4. Su VAULT, guarda cosa arriva.** Attiva *Impostazioni → Ricevitore*, apri la scheda **Ricevitore** e registra il repository in sola lettura.

!!! warning "La posizione è un percorso **dentro** il container, scritto relativo al mount dell'host"
    Inserisci `user/appdata/rest-server/tower/container`, **non** `/mnt/user/appdata/…`. BombVault gira in un container in cui il `/mnt` dell'host è montato altrove; un percorso host assoluto lì non esiste. Se ne incolli uno, BombVault ti indica il percorso relativo da usare.

    L'**APP_KEY di invio** è la chiave di TOWER, non quella di VAULT. La trovi su TOWER in *Impostazioni → Sistema*.

**5. Rendilo reciproco, se vuoi.** Ripeti gli stessi cinque passi nella direzione opposta: un rest-server su TOWER che riceve la copia di VAULT. Ogni macchina impone allora l'immutabilità all'altra, e nessuna può cancellare i backup dell'altra.

## Ripristino guidato

Una scheda **Ripristino** dedicata accompagna un'installazione pulita o ricostruita attraverso il caso di disastro, in un unico posto:

1. **Verifica che BombVault possa leggere i tuoi backup** (l'insidia della chiave di crittografia messa in primo piano).
2. **Ripristina le impostazioni di BombVault stesso**, così i percorsi di backup, le destinazioni off-site e le credenziali di cui il resto del flusso ha bisogno risultano precompilati. Legge il backup delle impostazioni dal luogo che la riga Auto-backup indica in **Salvato su**, oppure dalla copia dell'Auto-backup in **Copiato su**, e mostra quel luogo con il suo indirizzo; per leggere da un altro luogo, modifica prima la riga Auto-backup al passaggio 3. Il ripristino viene applicato tramite un auto-riavvio sul socket Docker, così il database delle impostazioni in esecuzione non viene mai sovrascritto sotto un handle aperto.
3. **Collega i tuoi backup esistenti** tramite le righe della scheda Domini: nella riga di ogni dominio, scegli in **Salvato su** il luogo in cui si trovano i suoi backup e in **Copiato su** i luoghi che contengono le sue copie. Un luogo che nessuna riga offre ancora, come una condivisione, un server o un bucket cloud, si collega con **Aggiungi luogo**, la stessa finestra di Impostazioni, Archiviazione. **Connetti e anteprima** controlla poi che i backup si possano leggere.
4. **Scopre** i container, le VM e i set di file memorizzati al suo interno.
5. **Li ripristina tutti** (lasciati fermi, così li avvii deliberatamente), con il tuo kit di ripristino a un clic di distanza.

!!! note "Le copie off-site attendono dopo una ricostruzione"
    Quando il passo 4 ricostruisce voci senza le vecchie impostazioni, la replica off-site di quei domini si mette in pausa finché la collocazione predefinita non viene confermata. Vedi [Collocazione per elemento](#placement).

!!! tip "Migrazione pianificata contro disastro"
    Il ripristino guidato ripristina le impostazioni di BombVault stesso da un backup. Per uno spostamento *pianificato* su una nuova macchina, puoi invece portare la tua configurazione direttamente con la scheda **Esporta e importa impostazioni** (un file JSON portatile). Vedi [Configurazione](configuration.md#portable-settings-export-and-import).

### Ripristino da un altro repo BombVault

Una scheda separata nella scheda **Ripristino** apre il repo di un'*altra* istanza BombVault (una condivisione montata sotto `/mnt`, o un URL remoto) con l'**`APP_KEY` di quell'istanza**, in una sessione monouso e in sola lettura. Sfoglia i container, le VM e i set di file memorizzati lì, scegli uno snapshot e ripristinalo, e l'oggetto ripristinato diventa un normale container, VM o set di file locale. Nulla viene mai scritto nell'altro repo, e le tue impostazioni di backup restano intatte (la sessione risiede in memoria e scade da sé). Spostare un container dal server A al server B non significa più ripuntare le impostazioni del tuo repo e riportarle indietro dopo. La federazione dal vivo server-a-server è esplicitamente fuori ambito; questa è una deliberata estrazione monouso.

## Kit di ripristino della chiave di crittografia

Questo è il pezzo che rende possibile il disaster recovery anche quando non c'è alcun BombVault in esecuzione.

Un clic scarica la **chiave master**, la **password restic derivata** e le **posizioni e comandi esatti del repo**, così puoi ripristinare direttamente con la CLI di restic su qualsiasi macchina. Un promemoria sulla Dashboard ti assilla finché non l'hai conservato.

!!! danger "Conserva il kit di ripristino fuori dal server"
    Il kit contiene il segreto che decifra i tuoi backup. Tienilo in un luogo sicuro e separato dal server (un password manager, una copia stampata in una cassaforte). Se perdi sia BombVault che `APP_KEY` senza kit di ripristino, i tuoi backup cifrati non possono essere recuperati.

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
