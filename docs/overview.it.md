# Panoramica

La panoramica è la prima pagina della barra laterale e quella da controllare ogni mattina. Risponde a due domande a colpo d'occhio: è tutto salvato, e c'è qualcosa che richiede la tua attenzione? Comincia con un'intestazione di stato e quattro schede, e altre schede aspettano in **Personalizza**.

![La panoramica dopo qualche giorno: l'intestazione di stato, Richiede attenzione con un avviso, la protezione di ogni sezione, le destinazioni e la cronologia.](assets/screenshots/overview/home-dark.png#only-dark)
![La panoramica dopo qualche giorno: l'intestazione di stato, Richiede attenzione con un avviso, la protezione di ogni sezione, le destinazioni e la cronologia.](assets/screenshots/overview/home-light.png#only-light)

## L'intestazione di stato {#status-head}

L'intestazione sta in cima alla pagina e ci resta sempre. Il suo anello mostra quanto è protetto, per esempio **41 su 43 protetti**, e la riga accanto dice come stanno le cose:

| L'intestazione dice | Cosa significa |
|---|---|
| **Tutto salvato** | Ogni elemento di ogni sezione attivata ha un backup aggiornato. |
| **Salvato, con note** | Tutto è salvato, e alcune esecuzioni hanno lasciato una nota, come file che non hanno potuto leggere. |
| **Non tutto è ancora protetto** | Alcuni elementi non hanno un backup aggiornato, per esempio perché aspettano il primo. |
| **3 cose richiedono attenzione** | Qualcosa è fallito o aspetta una decisione. La scheda **Richiede attenzione** lo elenca. |
| **Backup in corso** | Un backup è in corso. L'anello diventa un anello di avanzamento con le tappe **Arresto**, **Lettura** e **Avvio**, e un backup completo conta i suoi elementi mentre procede, per esempio **Backup in corso: 12 di 43**. **Vedi esecuzione** apre l'esecuzione. |
| **Copia off-site in corso** | È in corso una copia verso una destinazione. Non mostra alcuna percentuale, perché `restic copy` non ne riporta. |

Sotto la riga compaiono quando c'è stata l'ultima esecuzione e quando parte la prossima prevista.

## Richiede attenzione {#needs-you}

Questa scheda elenca ciò che dovresti guardare, il più recente per primo, e dice **Niente da fare.** quando l'elenco è vuoto. Ogni riga ha la sua via d'uscita:

- **Un backup fallito** nomina l'elemento e il motivo. **Vedi errori** mostra l'output di restic stesso, e **Riprova** avvia di nuovo il backup.
- **Copia off-site non riuscita** nomina la sezione e da quando. **Replica ora** riprova.
- **Anomalia su plex** porta al riscontro con **Controlla**. Finché un riscontro critico è aperto, la conservazione tiene i backup vecchi di quell'elemento.
- Gli elementi **Nuovo** che BombVault ha trovato e che nessun backup copre ancora, come un container appena installato, offrono **Inizia il backup** o **Non fare il backup**.
- **Un backup che ha rinunciato** perché la sua app è rimasta occupata per tutta l'attesa, vedi [Quando l'app è occupata](backups.md#app-busy).
- **Una replica ZFS fallita**, in parole semplici come che il server non ha il comando zfs, con l'output del server sotto **Mostra**.
- **I backup sono fermi: la chiave non si può più aprire** compare quando questo server ha adottato la chiave di un altro server e `APP_KEY` è stata cambiata dopo. Vedi [Risoluzione dei problemi](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Salva il tuo kit di ripristino** resta finché non hai scaricato il kit e premuto **L'ho conservato in modo sicuro**.

## Stato di protezione {#protection-status}

Ogni sezione attivata ha una riga con quattro controlli:

| Controllo | Diventa verde quando | Altre parole che puoi vedere |
|---|---|---|
| **Locale** | l'ultimo backup è aggiornato | nessun backup ancora, nessuna pianificazione, ultima esecuzione fallita, backup in ritardo, non ogni elemento è protetto, backup in scadenza |
| **Off-site** | l'ultima copia fuori sede è aggiornata | nessuna copia off-site, copia in corso, copia in ritardo, copia in pausa, nessuna copia ancora, replica fallita |
| **Protezione dall'eliminazione** | una prova di manomissione ha dimostrato che il lato remoto rifiuta le cancellazioni | nessuna, nessuna e rischio accettato, non ancora dimostrata, prova fallita, non dimostrata perché il suo controllo è disattivato |
| **Ripristino di prova** | un ripristino dalla copia off-site è riuscito | mai eseguito, in corso, fallito, in ritardo, solo manuale, non possibile per questa sezione |

Una sezione è verde solo sulla base di fatti verificati. Una copia su questo server non conta come off-site, e una destinazione che hai segnato **Accetta senza protezione dall'eliminazione** mostra un anello grigio invece di un punto ambra, così il colore della sezione segue gli altri suoi controlli.

Una sezione pianificata mostra **Ancora nessun backup** e resta neutra finché non arriva l'ora della sua prima esecuzione. Dopo mostra **In ritardo** e avvisa una volta. Una sezione senza nulla da salvare, perché nessuna pianificazione raggiunge i suoi elementi, mostra **Non pianificato** e non è mai in ritardo.

Un clic su una sezione apre la sua finestra con ogni controllo spiegato per esteso, gli elementi che conta e **Avvia il ripristino di prova** o **Ripeti il ripristino di prova**. Indica anche **Regola di potatura impostata** o **Nessuna regola di potatura**, e la regola di conservazione della cartella propria di una sezione conta quanto le sue regole off-site.

![La finestra di protezione della sezione Container con i suoi quattro controlli spiegati per esteso.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![La finestra di protezione della sezione Container con i suoi quattro controlli spiegati per esteso.](assets/screenshots/overview/protection-window-light.png#only-light)

## Destinazioni {#storage-locations}

Ogni destinazione compare qui come un recipiente con il suo livello di riempimento, la velocità con cui cresce, per esempio **In crescita di 4,2 GB/settimana**, e quando sarebbe piena a questo ritmo, per esempio **piena tra circa 31 settimane**. S3, B2 e i rest-server non riportano quanto spazio resta, quindi mostrano **Spazio libero sconosciuto** e solo la loro crescita. Un clic porta alla pagina della destinazione. Vedi [Destinazioni](offsite-recovery.md#destinations).

## Cronologia {#history}

**Cronologia** ha due schede. **Esecuzioni** elenca ogni esecuzione con il suo risultato, la durata e il motivo, e un clic la apre. **Registro** mostra ogni riga con la sua ora, come la scrive BombVault, e può essere ristretto a un giorno. Una riga diventa arancione quando un backup è andato a buon fine ma ha lasciato fuori qualcosa, e la riga nomina ciò che ha lasciato fuori.

## Altre schede in Personalizza {#customize}

**Personalizza** in cima alla pagina ti permette di disporre le schede. Trascina una scheda dalla sua barra per spostarla, ridimensionala dal punto nell'angolo in basso oppure nascondila. Le schede nascoste aspettano sotto **Schede nascoste**, e **Mostra** ne riporta una. Mentre le disponi le schede dondolano un poco, e **Fatto** termina. La disposizione viene salvata in questo browser.

Queste schede partono nascoste:

- **Anomalie**: i riscontri aperti, vedi [Anomalie](anomalies.md).
- **Senza backup**: tutto ciò che su questo server nessun backup pianificato copre, ciascuno con il suo motivo, come **mai aggiunto**, **non incluso nel calendario** o **tutte le destinazioni della sua sezione sono disattivate**. Legge i container da Docker stesso, quindi trova anche uno che BombVault non ha mai visto.
- **Calendario**: gli ultimi 90 giorni, un quadrato per giorno. Un clic su un giorno mostra le sue righe nel registro sotto **Cronologia**.
- **Archiviazione**: quanto occupa ogni sezione e quanto bene si deduplicano i suoi dati.
- **Panoramica**: lo snapshot più vecchio tra tutte le sezioni, quando parte il prossimo backup e come è andato l'ultimo.
- **Statistiche**: quanti container e VM ci sono, quanti errori e quanti piani sono attivi o in pausa.
- **Integrazione host**: ciò che BombVault raggiunge e di cui ha bisogno sull'host, come il socket Docker, libvirt, restic e rclone. Parti da qui quando qualcosa sembra non collegato.

![Modalità Personalizza: le schede si possono trascinare, ridimensionare e nascondere, e quelle nascoste aspettano sotto.](assets/screenshots/overview/customize-dark.png#only-dark)
![Modalità Personalizza: le schede si possono trascinare, ridimensionare e nascondere, e quelle nascoste aspettano sotto.](assets/screenshots/overview/customize-light.png#only-light)

## Come conta BombVault {#counting}

Ogni scheda e l'intestazione contano allo stesso modo: gli elementi delle sezioni attivate che sono ancora installati e che non hai segnato **Non fare il backup**. Un elemento conta come protetto solo finché una pianificazione lo raggiunge e ha un backup. Un vecchio backup da solo non conta, quindi un container che hai tolto dalla pianificazione compare come non protetto finché non lo rimetti o lo segni **Non fare il backup**.

## Primi passi {#first-steps}

In una nuova installazione la panoramica comincia con la scheda **Primi passi**: **Scegli le sezioni**, **Aggiungi destinazione** e **Primo backup**. Resta finché il primo backup non è terminato. [Primi passi](getting-started.md#first-run) la illustra.
