# Risoluzione dei problemi

Una breve FAQ. Per la tabella completa di risoluzione dei problemi lato host del backup VM via SSH (permission-denied, verifica della chiave host, variabili del template mancanti e altro), vedi la [guida al backup delle VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) su GitHub.

## Qualcosa non è collegato correttamente

Apri `/spike` nell'interfaccia web. La verifica dell'integrazione host sonda ogni mount e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala eventuali pezzi mancanti. Inizia da qui prima di dare per scontato un bug: un mount mancante o un host irraggiungibile si presenta immediatamente.

## Non riesco a raggiungere l'interfaccia web

BombVault serve HTTPS pronto all'uso sulla porta `3443` (certificato autofirmato), quindi apri `https://<your-unraid-ip>:3443`. Accetta l'avviso del certificato autofirmato, oppure metti BombVault dietro un reverse proxy con il tuo certificato. Se esegui con `HTTP_ONLY=true`, serve invece HTTP in chiaro sulla porta `3000` (pensato per l'uso dietro un proxy che termina il TLS).

## Ho perso la mia APP_KEY

`APP_KEY` deriva la password del repository restic. Senza di essa (e senza il kit di ripristino della chiave di crittografia), i backup cifrati non possono essere recuperati. Ecco perché la Dashboard ti assilla per scaricare il kit di ripristino. Vedi [Off-site e ripristino](offsite-recovery.md). Genera una chiave con `openssl rand -hex 32` e conservala fuori dal server prima di affidarti a qualsiasi backup.

## Il backup delle VM non si connette

Il backup delle VM comunica con libvirt via SSH, mai un mount.

- Conferma che SSH sia abilitato sull'host e che la chiave pubblica di BombVault sia autorizzata in `/root/.ssh/authorized_keys` (Impostazioni, Sistema, Backup VM via SSH mostra la chiave e un pulsante **Prova connessione**).
- Su una rete `br0.x` personalizzata, imposta `LIBVIRT_HOST` sull'IP LAN del tuo Unraid (lì il container non può raggiungere l'host tramite `host.docker.internal`). Abilita **Impostazioni, Docker, Host access to custom networks**.
- Se hai cambiato la porta SSH di Unraid, imposta `LIBVIRT_SSH_PORT` di conseguenza.
- La diagnosi completa passo passo (test di raggiungibilità, routing VLAN, `Permission denied (publickey)`, `Host key verification failed`) è nella [guida al backup delle VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Uno snapshot a caldo di una VM non è stato eseguito

Gli snapshot a caldo necessitano del qemu guest agent installato nella VM e del disco su `/mnt/cache` (o `/mnt/diskX`), non `/mnt/user`. Su una VM spenta, il metodo a caldo ripiega automaticamente su quello ordinato. Un backup ordinato spegne la VM, esegue il backup dei dischi, poi la riavvia, così è sempre consistente.

## Un backup è fallito con "repository is already locked"

Di solito è un lock restic orfano lasciato quando il container è stato aggiornato o riavviato a metà operazione. BombVault rileva un lock dimostrabilmente orfano, lo rimuove forzatamente e ritenta una volta, automaticamente. Se persiste, usa **Impostazioni, Integrità e manutenzione, Sblocca** per il dominio interessato per rimuovere a mano un lock bloccato. Un problema autentico si presenta comunque invece di essere nascosto.

## La mia copia off-site non è avvenuta dopo un backup

La replica off-site è best-effort per progettazione, così un intoppo off-site non fa mai fallire il backup locale. Controlla il calendario off-site per quel dominio (Impostazioni, Calendari): un calendario vuoto replica dopo ogni backup locale, mentre una cadenza spedisce meno spesso. Usa **Replica ora** nella scheda Off-site per un'esecuzione su richiesta, e osserva l'indicatore di replica sulla Dashboard.

## Un ripristino si è interrotto prima di iniziare

Prima che qualcosa venga fermato o rimosso, il ripristino esegue una verifica dei conflitti pre-volo: verifica che l'IP statico del container e le porte host pubblicate siano liberi. Se un altro container ne detiene già uno, si interrompe con un messaggio chiaro e utilizzabile invece di lasciare un ripristino a metà. Libera la porta o l'IP in conflitto, poi riprova.

## Un'esportazione in chiaro è fallita invece di scrivere un file

Se la cifratura age è attiva (Impostazioni) ma non è impostato alcun destinatario valido, un'esportazione fallisce con un errore chiaro invece di scrivere testo in chiaro. Aggiungi un destinatario valido (una chiave pubblica age o una chiave pubblica SSH), oppure disattiva la cifratura se intendi che l'esportazione sia in chiaro. Vedi [Funzionalità](features.md).

## Il container continua a riavviarsi o sembra non sano

BombVault segnala sano/non sano dal proprio `/api/health`. Uno strumento di auto-heal (come Autoheal) può riavviarlo automaticamente se il motore dovesse mai incepparsi. Controlla il log del container e il report `/spike` per la causa sottostante.

## Ancora bloccato?

- Leggi le pagine complete [Configurazione](configuration.md) e [Off-site e ripristino](offsite-recovery.md).
- Chiedi sul [thread di supporto Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Apri una [issue su GitHub](https://github.com/junkerderprovinz/bombvault/issues).
