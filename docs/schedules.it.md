# Pianificazioni

**Impostazioni, Calendario** contiene ogni piano: il backup completo che copre tutto, un piano proprio per una sezione, le copie off-site, quali elementi partecipano e in che ordine. Un singolo elemento può avere un piano proprio nella sua pagina. Questa pagina spiega come si incastrano e illustra una configurazione tipica.

## I piani si leggono come frasi {#plans}

Ogni piano è un pulsante che si legge come una frase, per esempio *ogni giorno alle 03:00* o *ogni 2 giorni alle 01:30*. Un clic lo apre con cinque modalità:

| Modalità | Cosa imposti |
|---|---|
| **Disattivato** | Su questo piano non gira nulla. |
| **Giornaliero** | **Ora** |
| **Settimanale** | **Giorni** e **Ora** |
| **Ogni N giorni** | **Ogni** tot **giorni**, e l'**Ora**. Conta dall'ultima esecuzione pianificata, quindi salvare un elemento a mano non sposta il piano in avanti. |
| **Cron** | Un'**Espressione** di cinque campi, come `0 */6 * * *` per ogni 6 ore. **Esempi** ne propone alcuni, e la finestra mostra le prossime esecuzioni. |

Tutti gli orari sono nel fuso orario del server. Su Unraid è quello del sistema. Altrove imposta `TZ`, altrimenti ogni piano gira in UTC, vedi [Variabili d'ambiente](configuration.md#environment-variables).

## Settimana {#week}

La scheda **Settimana** disegna ogni esecuzione prevista da lunedì a domenica, una riga per giorno: backup, copie off-site e controlli. Punta un tratto per leggere cosa parte lì. Dove tre piani partono entro due ore, il tratto è segnato, così puoi distanziarli.

![Impostazioni, Calendario: in cima la fascia Settimana, poi il backup completo e i piani per sezione.](assets/screenshots/schedule/week-dark.png#only-dark)
![Impostazioni, Calendario: in cima la fascia Settimana, poi il backup completo e i piani per sezione.](assets/screenshots/schedule/week-light.png#only-light)

## Backup completo {#full-backup}

Il **Backup completo** è il piano predefinito per tutto. Lo imposta **Quando vengono eseguiti i backup**: un unico passaggio che salva in sequenza ogni sezione, container, VM, flash, cartelle, dataset ZFS e per ultimo l'auto-backup. La riga sotto dice quali sezioni include. **Disattivato** lo spegne, e allora gira solo ciò che ha un piano proprio.

- **Recupera i backup mancati dopo l'avvio**: quando il server era spento a un orario previsto, BombVault esegue quel backup circa due minuti dopo l'avvio. Il piano proprio di un elemento e un piano off-site aspettano il loro prossimo orario previsto, e lo stesso vale per un piano che non è mai riuscito.
- **Comandi prima e dopo**: il **Comando prima del backup completo** gira sul server con `sh -c` prima che parta il passaggio, per esempio uno script che ferma un servizio. Se fallisce, il backup completo gira comunque. Il **Comando dopo il backup completo** gira una volta dopo che ogni sezione è stata tentata, anche se una è fallita, il che va bene per un ping che conferma che l'intero server è protetto. I comandi per un singolo container sono nella sua pagina.

**Esegui il backup di tutto ora** nella pagina Backup avvia a mano lo stesso passaggio, su ogni sezione attivata.

## Piani per sezione {#plans-per-section}

Una sezione può lasciare il backup completo e girare a un orario suo. In **Pianificazioni per sezione**, **Eccezione** aggiunge una sezione con un piano proprio, e **Rimuovi eccezione** la riporta indietro. La riga nella scheda dice cosa fanno le altre, per esempio **Tutte le altre sezioni vengono eseguite nel backup completo.**

Una sezione con un piano proprio non viene salvata di nuovo nel backup completo, così nulla viene salvato due volte. Lo stesso vale per un singolo elemento con un piano proprio.

**Copie off-site per sezione** funziona allo stesso modo per le copie. Di default ogni sezione copia sulle sue destinazioni off-site subito dopo ogni backup. **Eccezione** dà a una sezione un piano di copia proprio, per esempio perché le VM copino su Backblaze B2 solo la domenica sera. Anche la scheda **Copia** di una destinazione può impostare **Quando copiare**, vedi [Copie off-site](offsite-recovery.md#off-site-copies).

## Elementi pianificati {#scheduled-entries}

**Elementi pianificati** dice per sezione quanti elementi partecipano, per esempio **38 su 41 pianificati**. **Elementi** apre l'elenco con un interruttore **Nel calendario** per elemento. Un elemento disattivato viene salvato solo quando lo avvii a mano. Conta come non protetto a meno che tu lo segni **Non fare il backup**.

## Ordine {#order}

**Ordine** stabilisce l'ordine in cui le esecuzioni pianificate e il backup completo salvano container e VM. Trascina un elemento al suo posto o usa le frecce, e ogni spostamento viene salvato subito. Gli elementi che lasci senza ordine vengono salvati partendo dal più in ritardo. Un singolo backup a mano non ne è toccato.

**Riavvio dopo il backup** nella stessa scheda riguarda i container che **Arresta altri container** ha fermato durante un backup. Ripartono nel loro ordine di dipendenza, e con **Attendere che le dipendenze siano integre prima di avviare il successivo** ciascuno deve risultare sano o in esecuzione prima che parta il successivo. Il **Timeout di integrità per container (secondi)** limita l'attesa, 120 di default, in un intervallo da 5 a 3600.

## Snapshot invece dello stop {#snapshot-instead-of-stop}

**Ferma i container su ZFS solo per uno snapshot** è attivo di default. Un container i cui dati stanno su ZFS viene fermato solo mentre BombVault fa uno snapshot di quei dati, di solito pochi secondi, e il backup viene poi letto dallo snapshot. Richiede la connessione SSH all'host che usano i dataset ZFS. Un container con una parte dei dati altrove viene fermato per tutto il backup come prima, e la sua esecuzione ne dice il motivo, mentre un container senza alcun dato su ZFS viene fermato senza una nota. Funziona su una condivisione utente di Unraid che sta su un solo pool ZFS. Su TrueNAS l'app richiede anche la capability `SYS_CHROOT`, vedi [TrueNAS SCALE](getting-started.md#truenas). Un singolo container può essere impostato diversamente nella sua pagina.

Il backup finisce sotto gli stessi percorsi e tag di uno fatto con il container fermo, quindi ripristini e verifiche li trattano allo stesso modo. Il comando prima del backup e il dump del database girano comunque per primi, gli altri container da fermare si fermano e ripartono con esso, e il comando dopo il backup gira non appena il container è di nuovo attivo. Un aggiornamento dopo il backup aspetta comunque che il backup sia finito. Le sue esecuzioni mostrano **Snapshot** e per quanti secondi il container è rimasto fermo. Se BombVault si ferma nel mezzo di un qualsiasi backup di container, al successivo avvio fa ripartire i container che teneva fermi.

## Aggiornamenti dopo il backup {#updates}

- **Aggiorna i container dopo un backup riuscito** scarica un'immagine più recente solo dopo che il backup precedente è finito, così esiste sempre prima un punto di ripristino fresco. È disattivato di default, e un singolo container può essere impostato diversamente nella sua pagina sotto **Aggiorna dopo un backup riuscito**. Alla nuova immagine passa solo ciò che hai impostato tu: variabili, label, comando, utente e healthcheck ancora uguali a quelli della vecchia immagine prendono quelli della nuova. Se la nuova immagine non si può creare o avviare, il container torna all'immagine con cui girava, l'aggiornamento risulta fallito e il backup successivo riprova. Un container rimasto fermo per questo viene nominato nell'esecuzione del backup e in una notifica.
- **Rimuovi la vecchia image dopo un aggiornamento** elimina l'immagine che il container ha lasciato. Un'immagine di base condivisa con altri container non viene mai eliminata.
- **Aggiorna lo stato degli aggiornamenti di Unraid dopo aver aggiornato un container** cancella il banner degli aggiornamenti di Unraid per quel container.

Un messaggio per ogni container aggiornato si imposta in **Impostazioni, Notifiche** con **Notifica all'aggiornamento del container**.

## Inattività prima del backup {#idle}

La scheda **Inattività prima del backup** stabilisce quando un'app conta come inattiva per un container il cui backup pianificato la aspetta: **CPU sotto (% di un core)**, **Traffico sotto (Mbit/s)** e **Per almeno (min)**. Un media server conta come inattivo quando non fa streaming, e per un container sulla rete host conta solo la CPU. L'attesa in sé si imposta su ogni container, vedi [Quando l'app è occupata](backups.md#app-busy).

## Un piano proprio per un elemento {#own-plan}

Ogni elemento ha **Piano proprio** nella scheda **Calendario e destinazione** della sua scheda Impostazioni. Con un piano lì, l'elemento gira ai suoi orari e lascia il backup completo e il piano della sua sezione. Senza, li segue.

## Esempio: tre piani che si completano {#example}

Un tipico server Unraid: la maggior parte dei container si può salvare di notte, le VM sono grandi e cambiano lentamente, e Immich riceve nuove foto tutto il giorno.

1. **Backup completo alle 03:00.** Apri **Impostazioni, Calendario**. In **Backup completo** imposta **Quando vengono eseguiti i backup** su **Giornaliero** alle `03:00`. Attiva **Recupera i backup mancati dopo l'avvio**, così una notte con il server spento viene recuperata.
2. **VM una volta a settimana.** In **Pianificazioni per sezione** premi **Eccezione**, scegli **VM** e imposta il suo piano su **Settimanale**, **Giorni** domenica, **Ora** `04:00`. La scheda dice ora che tutte le altre sezioni vengono eseguite nel backup completo.
3. **Anche le VM copiano off-site la domenica.** In **Copie off-site per sezione** premi **Eccezione**, scegli **VM** e imposta **Settimanale**, domenica alle `06:00`, dopo che il loro backup ha avuto il tempo di finire.
4. **Immich ogni 6 ore.** Apri **Backup**, poi `immich`, poi la sua scheda **Impostazioni**. Imposta **Piano proprio** su **Cron** con `0 */6 * * *`.

Cosa gira ora:

| Quando | Cosa |
|---|---|
| Ogni giorno alle 03:00 | I container tranne `immich`, flash, cartelle, dataset ZFS e l'auto-backup, una sezione dopo l'altra |
| Domenica alle 04:00 | VM |
| Domenica alle 06:00 | La copia off-site delle VM |
| 00:00, 06:00, 12:00 e 18:00 | `immich` per conto suo |

La scheda **Settimana** mostra la riga delle 03:00 ogni giorno, i tratti delle 04:00 e delle 06:00 la domenica, e quattro tratti al giorno per `immich`. `immich` non viene salvato una seconda volta alle 03:00, perché un elemento con un piano proprio lascia il backup completo. Le copie delle altre sezioni girano comunque subito dopo ogni backup.

Quando in seguito rimuovi il piano proprio di `immich`, rientra nell'esecuzione delle 03:00 con la sua sezione.
