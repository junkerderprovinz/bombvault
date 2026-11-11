# App Android

L'app Android porta sul telefono tutti i server BombVault del tuo gruppo. Si apre su un elenco dei tuoi server, con in cima il registro attività di tutti, le stesse righe che mostra la panoramica, e apre la vista per telefono del server che tocchi. L'app non salva nulla da sola.

![La schermata iniziale dell'app: in cima il registro attività di tutti i server, poi l'elenco dei server.](assets/screenshots/android/start-dark.png#only-dark)
![La schermata iniziale dell'app: in cima il registro attività di tutti i server, poi l'elenco dei server.](assets/screenshots/android/start-light.png#only-light)

## Come ottenere l'app {#install}

- **Impostazioni, App:** la scheda dell'app Android offre l'APK per la release che gira sul tuo server, con un codice QR da inquadrare con il telefono, e rimanda alla scheda su F-Droid.
- **APK:** ogni release ha `bombvault-android.apk` sulla sua pagina, e [questo link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) scarica sempre l'ultima versione. Serve Android 10 o successivo, e Android chiede una volta se l'app con cui apri il file può installare app.
- **Google Play:** l'app è in un test chiuso finché non potrà essere pubblica. Google Play mostra l'app di un nuovo account sviluppatore solo dopo che almeno 12 tester l'hanno tenuta installata per 14 giorni. Per dare una mano, entra nel [gruppo dei tester](https://groups.google.com/g/arrowloop-testers), apri la [pagina di test](https://play.google.com/apps/testing/bombvault.halleluja.design), tocca **Diventa un tester** e installa BombVault da Google Play.
- **F-Droid:** [BombVault su F-Droid](https://f-droid.org/packages/bombvault.halleluja.design/). F-Droid compila l'app dal sorgente e la firma con la propria chiave, quindi una nuova release gli arriva dopo qualche giorno. Per passare dall'app F-Droid all'APK, disinstalla prima l'altra.

Associa l'app a server con la versione 9.7.0 o successiva. Un server con una versione più vecchia può stare nello stesso gruppo, ma mostra il telefono come una semplice istanza, e l'app legge l'attività di quel server solo dopo un accesso.

## Associazione con il codice QR {#pairing}

1. Su un server qualsiasi del tuo gruppo apri **Impostazioni, Associazione** e scegli **Mostra frase**. Compaiono le dodici parole con un codice QR accanto.
2. Nell'app tocca **Inquadra codice QR** e punta il telefono sul codice. Puoi anche incollare o digitare le parole.
3. L'app elenca i server di quel gruppo prima di salvare qualsiasi cosa. **Aggiungi tutte** le aggiunge tutte.

Il telefono entra poi nel gruppo come un'altra istanza. Legge ciò che gira su ogni server attraverso il gruppo senza un accesso, a casa direttamente e fuori casa tramite il relay. Come funziona il gruppo in sé è descritto in [Associare le istanze](offsite-recovery.md#pairing).

L'app entra nel gruppo tramite il relay, anche a casa. Se le tue istanze usano **Nessun relay**, scegline uno in **Impostazioni, Associazione** prima di inquadrare il codice.

## Fuori casa {#away}

L'app prova prima un server al suo indirizzo. Quando non risponde, come sulla rete mobile, apre invece l'interfaccia del server tramite il relay del gruppo. Un server del gruppo di cui l'app non conosce ancora l'indirizzo si apre allo stesso modo. Se né l'indirizzo né il relay rispondono, la schermata iniziale lo dice sulla scheda del server.

Tramite il relay la pagina raggiunge ciò che un telefono associato raggiunge a casa, come descritto più sotto. L'accesso con la password e il salvataggio di file funzionano solo all'indirizzo del server. Le barre di avanzamento si aggiornano ogni due secondi invece che in continuo. I file dell'interfaccia restano sul telefono dopo la prima visita, così le visite successive si caricano più in fretta. Un server più vecchio della 10.0.0 si apre solo al suo indirizzo.

L'app accetta una risposta solo se il server l'ha firmata con la chiave che l'app ha fissato quando ha incontrato quel server per la prima volta. Se in seguito il server risponde con un'altra chiave, la schermata iniziale mostra l'impronta della nuova chiave, e la pagina si apre solo dopo che ti sei fidato.

## Cambiare server {#switch}

Con un server aperto, in cima alla sua panoramica compare il nome del server, preceduto da una freccia indietro. La freccia porta alla panoramica dell'app, la schermata iniziale, che elenca ogni server noto all'app con il suo stato: **Collegato**, **Non collegato**, **Accesso necessario** o **Verifica certificato**. Un server del gruppo che non risponde è **Inattiva**, in grigio, con l'indicazione di quando il gruppo l'ha visto per l'ultima volta, e uno da cui l'app non ha ancora ricevuto nulla dice **Controllo in corso**. Ogni volta che si collega, l'app legge l'elenco del gruppo da una delle sue istanze, quindi viene elencata anche un'istanza mai incontrata sul relay, per esempio una spenta quando il telefono è entrato. Tocca un altro server per aprirlo, direttamente a casa e altrimenti tramite il relay.

## Accesso su un telefono associato {#sign-in}

Un telefono associato al tuo gruppo apre ciascuno dei suoi server già con l'accesso effettuato. Prima di caricare una pagina chiede a quel server, attraverso il gruppo, una sessione, e un server la consegna solo a un membro che è un telefono.

Un telefono associato non ottiene tutto ciò che ottiene una password. La sua sessione raggiunge un elenco fisso di ciò che l'app mostra: la panoramica, le esecuzioni e il registro attività, i backup da consultare, avvio e ripristino, le anomalie da leggere e la pagina Istanze da consultare, senza i repository ricevuti e le sorgenti di prelievo dei suoi membri. Destinazioni e comandi tornano senza indirizzi e segreti. Un ripristino che sostituisce l'originale esegue prima il backup dello stato attuale. Un ripristino in una cartella finisce in una nuova cartella dentro la cartella di ripristino impostata in **Impostazioni, Ripristino**, e la finestra non ne offre altre. Chiavi, kit di ripristino, depositi della chiave, impostazioni ed eliminazione dei backup restano legati a un accesso sul server stesso, e tutto il resto risponde che serve quell'accesso. Tramite il relay vale lo stesso elenco, anche su un server senza password.

L'interruttore per questo è **I telefoni possono associarsi con le dodici parole del gruppo** nella scheda **App Android** in **Impostazioni, Sicurezza**. Un reset della password lo disattiva e fa uscire ogni telefono, finché non lo riattivi lì.

## Certificati {#certificates}

Un certificato autofirmato viene considerato attendibile una volta, tramite la sua impronta SHA-256. Quando in seguito il server ne mostra uno diverso, l'app ti avvisa e lo apre solo dopo che ti fidi del nuovo certificato.

## Il telefono nella pagina Istanze {#instances}

Il telefono ha una scheda propria nella pagina Istanze di ogni server del gruppo, contrassegnata come app Android e col nome che hai dato al telefono. Non ha un riepilogo, perché non salva nulla. **Rimuovi** la toglie dalla pagina e termina subito la sua sessione, come fa anche disattivare **Interroga questa istanza** sotto **Monitoraggio** nei suoi dettagli. Il telefono conosce ancora le parole, quindi riappare quando si apre la sua app, ma disattivato, e non riceve una sessione finché qualcuno non lo riattiva. Per un telefono perso rimuovi la sua scheda su ogni server, oppure disattiva **I telefoni possono associarsi con le dodici parole del gruppo** in **Impostazioni, Sicurezza**, oppure fai **Lascia il gruppo** a ogni istanza e associale di nuovo con parole nuove.

Un telefono associato prima della 10.0.0 comincia a firmare le sue chiamate alla prima visita dopo l'aggiornamento, e ogni server tiene la prima chiave che riceve per il telefono. Se qualcun altro che conosce le parole del gruppo arriva prima mentre il telefono è offline, quel server respinge da quel momento il telefono, e l'app non riesce più ad aprirlo tramite il relay. Per rimediare, accedi a quel server in un browser, scegli **Dimentica chiave** sulla scheda del telefono nella pagina Istanze e poi **Conferma di dimenticare**, e riapri l'app. Il server prende la chiave del telefono alla chiamata successiva. Se il telefono viene respinto di nuovo, qualcun altro ha le parole: fai **Lascia il gruppo** a ogni istanza e associale di nuovo con parole nuove, come per un telefono perso.

## Impostazioni {#settings}

L'ingranaggio accanto al più apre le impostazioni dell'app:

- la lingua e il nome che il telefono mostra nella pagina Istanze (vuoto significa il modello del telefono),
- l'aspetto, che segue il primo server dell'elenco finché non ne imposti uno tuo, e le animazioni, che hanno un'impostazione a parte,
- un rapporto da copiare quando segnali un problema. Non contiene indirizzi, nomi né frasi,
- la scheda Informazioni con l'informativa sulla privacy,
- **Rimuovi tutti i server**, che toglie dall'app ogni server e lascia il gruppo. Ogni server che il telefono raggiunge ancora lo toglie dalla sua pagina Istanze.

## Download e upload {#files}

Esportazioni, kit di ripristino, ZIP della flash e dump dei database finiscono nella cartella Download del telefono, come farebbero da un browser. L'importazione delle impostazioni apre il selettore di file del telefono. Tramite il relay non viene salvato alcun file. Per questo apri il server al suo indirizzo.

## Su uno schermo touch {#touch}

Sotto un dito nulla scorre in hover, quindi un controllo si attenua finché lo tieni premuto, e un pulsante con il logo di un marchio si illumina nel colore del marchio finché il dito non si solleva. Una pressione lunga su un pulsante vale come un tocco lento e non apre alcun menu dei link. Un tocco su un pulsante disattivato mostra perché lo è, finché non tocchi altrove.

## In un browser {#browser}

Chrome ed Edge possono installare l'interfaccia web di BombVault come app in una finestra propria, sul telefono come sul computer. Non viene messo in cache nulla, quindi un aggiornamento si vede subito.

## Privacy {#privacy}

L'app non ha account, pubblicità né analytics, e non esegue nulla in background. La sua [informativa sulla privacy](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) elenca cosa memorizza e cosa invia e dove.
